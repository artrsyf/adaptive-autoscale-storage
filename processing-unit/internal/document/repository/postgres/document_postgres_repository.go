package postgres

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"autoscale-distr-storage/processing-unit/internal/document/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

type DocumentPostgresRepository struct {
	pool            *pgxpool.Pool
	metrics         *DocumentPostgresMetrics
	rollbackTimeout time.Duration
}

// Open настраивает пул, таймауты PostgreSQL и метрики; доступность схемы отдельно проверяет Ready.
func Open(operationContext context.Context, config DocumentPostgresConfig, password, node string, registry *prometheus.Registry) (*DocumentPostgresRepository, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	documentPostgresMetrics := NewDocumentPostgresMetrics(registry, node)
	processingUnitConfig, err := pgxpool.ParseConfig(config.DSN(password))
	if err != nil {
		return nil, err
	}
	processingUnitConfig.MaxConns = config.PoolMax
	processingUnitConfig.MinConns = 0
	processingUnitConfig.ConnConfig.RuntimeParams["application_name"] = node
	processingUnitConfig.ConnConfig.RuntimeParams["statement_timeout"] = strconv.FormatInt(config.StatementTimeout.Time().Milliseconds(), 10)
	processingUnitConfig.ConnConfig.RuntimeParams["idle_in_transaction_session_timeout"] = strconv.FormatInt(config.IdleTransactionTimeout.Time().Milliseconds(), 10)
	withConfig, err := pgxpool.NewWithConfig(operationContext, processingUnitConfig)
	if err != nil {
		return nil, err
	}
	documentPostgresRepository := &DocumentPostgresRepository{pool: withConfig, metrics: documentPostgresMetrics, rollbackTimeout: config.RollbackTimeout.Time()}
	for name, collectMetric := range map[string]func() float64{
		"acquired": func() float64 { return float64(withConfig.Stat().AcquiredConns()) },
		"idle":     func() float64 { return float64(withConfig.Stat().IdleConns()) },
		"max":      func() float64 { return float64(withConfig.Stat().MaxConns()) },
	} {
		registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{Name: "storage_pool_connections", Help: "processing unit connection pool size by state.", ConstLabels: prometheus.Labels{"node": node, "state": name}}, collectMetric))
	}
	return documentPostgresRepository, nil
}

const columns = `partition_key,id,payload,revision::text,created_at,updated_at`

// scan читает строку SQL в модель документа и возвращает ошибку драйвера без преобразования.
func scan(row pgx.Row) (document.Record, error) {
	var documentRecord document.Record
	var payload []byte
	err := row.Scan(&documentRecord.PartitionKey, &documentRecord.ID, &payload, &documentRecord.Revision, &documentRecord.CreatedAt, &documentRecord.UpdatedAt)
	documentRecord.Payload = document.Payload(payload)
	return documentRecord, err
}

// Close закрывает пул PostgreSQL и освобождает его соединения.
func (documentPostgresRepository *DocumentPostgresRepository) Close() {
	documentPostgresRepository.pool.Close()
}

// Ready проверяет доступность БД и наличие таблицы документов.
func (documentPostgresRepository *DocumentPostgresRepository) Ready(operationContext context.Context) error {
	var exists bool
	if err := documentPostgresRepository.pool.QueryRow(operationContext, `SELECT to_regclass('public.documents') IS NOT NULL`).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("schema missing")
	}
	return nil
}

// observe измеряет получение соединения отдельно от SQL и переводит известные ошибки PostgreSQL в ошибки приложения.
func (documentPostgresRepository *DocumentPostgresRepository) observe(operationContext context.Context, operation string, query func(*pgxpool.Conn) (document.Record, error)) (record document.Record, err error) {
	start := time.Now()
	connection, operationError := documentPostgresRepository.pool.Acquire(operationContext)
	outcome := "ok"
	if operationError != nil {
		outcome = "error"
	}
	documentPostgresRepository.metrics.PoolWait.WithLabelValues(operation, outcome).Observe(time.Since(start).Seconds())
	if operationError != nil {
		return record, operationError
	}
	defer connection.Release()
	start = time.Now()
	defer func() {
		if operationContext.Err() != nil && err != nil {
			err = operationContext.Err()
		}
		var pgError *pgconn.PgError
		if errors.As(err, &pgError) {
			switch pgError.Code {
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
		documentPostgresRepository.metrics.DBDuration.WithLabelValues(operation, outcome).Observe(time.Since(start).Seconds())
	}()
	return query(connection)
}

// Create вставляет документ с новой revision; дубликат ключа преобразуется в ErrExists.
func (documentPostgresRepository *DocumentPostgresRepository) Create(operationContext context.Context, key document.Key, payload document.Payload) (document.Record, error) {
	return documentPostgresRepository.observe(operationContext, "create", func(connection *pgxpool.Conn) (document.Record, error) {
		return scan(connection.QueryRow(operationContext, `INSERT INTO documents(partition_key,id,payload) VALUES($1,$2,$3) RETURNING `+columns, key.PartitionKey, key.ID, []byte(payload)))
	})
}

// Get читает документ по составному ключу; отсутствие строки возвращает ErrNotFound.
func (documentPostgresRepository *DocumentPostgresRepository) Get(operationContext context.Context, key document.Key) (document.Record, error) {
	return documentPostgresRepository.observe(operationContext, "get", func(connection *pgxpool.Conn) (document.Record, error) {
		record, err := scan(connection.QueryRow(operationContext, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2`, key.PartitionKey, key.ID))
		if errors.Is(err, pgx.ErrNoRows) {
			err = document.ErrNotFound
		}
		return record, err
	})
}

// Replace заменяет payload и revision под блокировкой строки после проверки ожидаемой версии.
func (documentPostgresRepository *DocumentPostgresRepository) Replace(operationContext context.Context, key document.Key, payload document.Payload, expectedRevision string) (document.Record, error) {
	return documentPostgresRepository.observe(operationContext, "update", func(connection *pgxpool.Conn) (document.Record, error) {
		return documentPostgresRepository.lockedMutation(operationContext, connection, key, expectedRevision, func(transaction pgx.Tx, _ document.Record) (document.Record, error) {
			return scan(transaction.QueryRow(operationContext, `UPDATE documents SET payload=$3,revision=gen_random_uuid(),updated_at=clock_timestamp() WHERE partition_key=$1 AND id=$2 RETURNING `+columns, key.PartitionKey, key.ID, []byte(payload)))
		})
	})
}

// Delete удаляет документ после проверки revision под блокировкой; возвращает метаданные без payload.
func (documentPostgresRepository *DocumentPostgresRepository) Delete(operationContext context.Context, key document.Key, expectedRevision string) (document.Record, error) {
	return documentPostgresRepository.observe(operationContext, "delete", func(connection *pgxpool.Conn) (document.Record, error) {
		return documentPostgresRepository.lockedMutation(operationContext, connection, key, expectedRevision, func(transaction pgx.Tx, record document.Record) (document.Record, error) {
			_, err := transaction.Exec(operationContext, `DELETE FROM documents WHERE partition_key=$1 AND id=$2`, key.PartitionKey, key.ID)
			record.Payload = nil
			return record, err
		})
	})
}

// lockedMutation блокирует строку, сверяет revision и выполняет изменение в одной транзакции; возвращает результат после commit.
func (documentPostgresRepository *DocumentPostgresRepository) lockedMutation(operationContext context.Context, connection *pgxpool.Conn, key document.Key, expectedRevision string, mutate func(pgx.Tx, document.Record) (document.Record, error)) (document.Record, error) {
	transaction, err := connection.Begin(operationContext)
	if err != nil {
		return document.Record{}, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), documentPostgresRepository.rollbackTimeout)
		defer cancel()
		_ = transaction.Rollback(cleanup)
	}()
	record, err := scan(transaction.QueryRow(operationContext, `SELECT `+columns+` FROM documents WHERE partition_key=$1 AND id=$2 FOR UPDATE`, key.PartitionKey, key.ID))
	if errors.Is(err, pgx.ErrNoRows) {
		return document.Record{}, document.ErrNotFound
	}
	if err != nil {
		return document.Record{}, err
	}
	if record.Revision != expectedRevision {
		return document.Record{}, document.ErrConflict
	}
	record, err = mutate(transaction, record)
	if err != nil {
		return document.Record{}, err
	}
	if err = transaction.Commit(operationContext); err != nil {
		return document.Record{}, err
	}
	return record, nil
}
