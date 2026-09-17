package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"autoscale-distr-storage/internal/document/domain"
	"autoscale-distr-storage/internal/platform/telemetry"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type Repository struct {
	pool    *pgxpool.Pool
	metrics *telemetry.Metrics
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
	var payload []byte
	err := row.Scan(&r.PartitionKey, &r.ID, &payload, &r.Revision, &r.CreatedAt, &r.UpdatedAt)
	r.Payload = document.Payload(payload)
	return r, err
}

func (r *Repository) Close() { r.pool.Close() }

func (r *Repository) Ready(ctx context.Context) error {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT to_regclass('public.documents') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("schema missing")
	}
	return nil
}

func (r *Repository) observe(ctx context.Context, op string, query func(*pgxpool.Conn) (document.Record, error)) (record document.Record, err error) {
	start := time.Now()
	conn, e := r.pool.Acquire(ctx)
	outcome := "ok"
	if e != nil {
		outcome = "error"
	}
	r.metrics.PoolWait.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
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
		r.metrics.DBDuration.WithLabelValues(op, outcome).Observe(time.Since(start).Seconds())
	}()
	return query(conn)
}

func (r *Repository) Create(ctx context.Context, key document.Key, payload document.Payload) (document.Record, error) {
	return r.observe(ctx, "create", func(conn *pgxpool.Conn) (document.Record, error) {
		return scan(conn.QueryRow(ctx, `INSERT INTO documents(partition_key,id,payload) VALUES($1,$2,$3) RETURNING `+columns, key.PartitionKey, key.ID, []byte(payload)))
	})
}
func (r *Repository) Get(ctx context.Context, key document.Key) (document.Record, error) {
	return r.observe(ctx, "get", func(conn *pgxpool.Conn) (document.Record, error) {
		record, err := scan(conn.QueryRow(ctx, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2`, key.PartitionKey, key.ID))
		if errors.Is(err, pgx.ErrNoRows) {
			err = document.ErrNotFound
		}
		return record, err
	})
}
func (r *Repository) Replace(ctx context.Context, key document.Key, payload document.Payload, expectedRevision string) (document.Record, error) {
	return r.observe(ctx, "update", func(conn *pgxpool.Conn) (document.Record, error) {
		return lockedMutation(ctx, conn, key, expectedRevision, func(tx pgx.Tx, _ document.Record) (document.Record, error) {
			return scan(tx.QueryRow(ctx, `UPDATE documents SET payload=$3,revision=gen_random_uuid(),updated_at=clock_timestamp() WHERE partition_key=$1 AND id=$2 RETURNING `+columns, key.PartitionKey, key.ID, []byte(payload)))
		})
	})
}
func (r *Repository) Delete(ctx context.Context, key document.Key, expectedRevision string) (document.Record, error) {
	return r.observe(ctx, "delete", func(conn *pgxpool.Conn) (document.Record, error) {
		return lockedMutation(ctx, conn, key, expectedRevision, func(tx pgx.Tx, record document.Record) (document.Record, error) {
			_, err := tx.Exec(ctx, `DELETE FROM documents WHERE partition_key=$1 AND id=$2`, key.PartitionKey, key.ID)
			record.Payload = nil
			return record, err
		})
	})
}

// Lock and compare at one serialization point; no unlocked pre-read in the service.
func lockedMutation(ctx context.Context, conn *pgxpool.Conn, key document.Key, expectedRevision string, mutate func(pgx.Tx, document.Record) (document.Record, error)) (document.Record, error) {
	tx, err := conn.Begin(ctx)
	if err != nil {
		return document.Record{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	record, err := scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2 FOR UPDATE`, key.PartitionKey, key.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return document.Record{}, document.ErrNotFound
	}
	if err != nil {
		return document.Record{}, err
	}
	if record.Revision != expectedRevision {
		return document.Record{}, document.ErrConflict
	}
	record, err = mutate(tx, record)
	if err != nil {
		return document.Record{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return document.Record{}, err
	}
	return record, nil
}
