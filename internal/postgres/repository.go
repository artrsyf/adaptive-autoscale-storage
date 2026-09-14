package postgres

import (
	"context"
	"errors"
	"time"

	"autoscale-distr-storage/internal/document"
	"autoscale-distr-storage/internal/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type Repository struct {
	Pool    *pgxpool.Pool
	Metrics *telemetry.Metrics
}

func Open(ctx context.Context, dsn, node string, max int32, m *telemetry.Metrics) (*Repository, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = max
	cfg.MinConns = 0
	cfg.ConnConfig.RuntimeParams["application_name"] = node
	cfg.ConnConfig.RuntimeParams["statement_timeout"] = "2500"
	cfg.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = "5000"
	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	r := &Repository{p, m}
	for name, fn := range map[string]func() float64{
		"acquired": func() float64 { return float64(p.Stat().AcquiredConns()) },
		"idle":     func() float64 { return float64(p.Stat().IdleConns()) },
		"max":      func() float64 { return float64(p.Stat().MaxConns()) },
	} {
		m.Registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_pool_connections", Help: "PU connection pool size by state.", ConstLabels: prometheus.Labels{"node": node, "state": name}}, fn))
	}
	return r, nil
}

const columns = `partition_key,id,payload,revision::text,created_at,updated_at`

func scan(row pgx.Row) (document.Record, error) {
	var r document.Record
	err := row.Scan(&r.PartitionKey, &r.ID, &r.Payload, &r.Revision, &r.CreatedAt, &r.UpdatedAt)
	return r, err
}

func (r *Repository) Execute(ctx context.Context, op string, c document.Command) (record document.Record, err error) {
	if err = c.Validate(op); err != nil {
		return
	}
	start := time.Now()
	conn, e := r.Pool.Acquire(ctx)
	outcome := "ok"
	if e != nil {
		outcome = "error"
	}
	r.Metrics.PoolWait.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
	if e != nil {
		return record, e
	}
	defer conn.Release()
	start = time.Now()
	defer func() {
		if ctx.Err() != nil && err != nil {
			err = ctx.Err()
		}
		var pe *pgconn.PgError
		if errors.As(err, &pe) {
			switch pe.Code {
			case "23505":
				err = document.ErrExists
			case "22P02", "22021", "22P05", "23514":
				err = document.ErrInvalid
			case "57014":
				err = context.DeadlineExceeded
			}
		}
		outcome := "ok"
		if err != nil {
			outcome = "error"
		}
		r.Metrics.DBDuration.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
	}()
	switch op {
	case "create":
		record, err = scan(conn.QueryRow(ctx, `INSERT INTO documents(partition_key,id,payload) VALUES($1,$2,$3) RETURNING `+columns, c.PartitionKey, c.ID, c.Payload))
	case "get":
		record, err = scan(conn.QueryRow(ctx, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2`, c.PartitionKey, c.ID))
		if errors.Is(err, pgx.ErrNoRows) {
			err = document.ErrNotFound
		}
	case "update", "delete":
		// Lock the row to distinguish missing records from version conflicts at a
		// single serialization point. A timeout rolls the transaction back.
		var tx pgx.Tx
		tx, err = conn.Begin(ctx)
		if err != nil {
			return
		}
		defer func() {
			cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = tx.Rollback(cleanup)
		}()
		record, err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2 FOR UPDATE`, c.PartitionKey, c.ID))
		if errors.Is(err, pgx.ErrNoRows) {
			err = document.ErrNotFound
			return
		}
		if err != nil {
			return
		}
		if record.Revision != c.ExpectedRevision {
			err = document.ErrConflict
			return
		}
		if op == "update" {
			record, err = scan(tx.QueryRow(ctx, `UPDATE documents SET payload=$3,revision=gen_random_uuid(),updated_at=clock_timestamp() WHERE partition_key=$1 AND id=$2 RETURNING `+columns, c.PartitionKey, c.ID, c.Payload))
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM documents WHERE partition_key=$1 AND id=$2`, c.PartitionKey, c.ID)
			record.Payload = nil
		}
		if err != nil {
			return
		}
		err = tx.Commit(ctx)
	}
	return
}
