package postgres

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"autoscale-distr-storage/processing-unit/internal/utils"
)

type DocumentPostgresConfig struct {
	// Host — DNS-имя или IP PostgreSQL; в Compose соответствует имени сервиса базы.
	Host string `yaml:"host"`
	// Port — TCP-порт сервера; допустимы значения от 1 до 65535.
	Port int `yaml:"port"`
	// Database — Имя базы PostgreSQL, созданной при инициализации стенда.
	Database string `yaml:"database"`
	// User — Роль PostgreSQL для запросов приложения; пароль в YAML не хранится.
	User string `yaml:"user"`
	// SSLMode — Режим TLS PostgreSQL: disable, allow, prefer, require, verify-ca или verify-full.
	SSLMode string `yaml:"sslmode"`
	// PoolMax — Максимум соединений PostgreSQL на один processing unit; бюджеты всех нод суммируются.
	PoolMax int32 `yaml:"pool_max"`
	// StatementTimeout — Серверный предел времени одного SQL statement, включая ожидание блокировок; не менее 1ms.
	StatementTimeout utils.Duration `yaml:"statement_timeout"`
	// IdleTransactionTimeout — Срок простоя открытой транзакции, после которого PostgreSQL завершает сессию; не менее 1ms.
	IdleTransactionTimeout utils.Duration `yaml:"idle_transaction_timeout"`
	// RollbackTimeout — Отдельный срок rollback при очистке транзакции, даже если контекст запроса уже отменён.
	RollbackTimeout utils.Duration `yaml:"rollback_timeout"`
}

// DSN собирает строку подключения с экранированием пароля; результат содержит секрет и не должен логироваться.
func (documentPostgresConfig DocumentPostgresConfig) DSN(password string) string {
	uRL := url.URL{Scheme: "postgres", Host: net.JoinHostPort(documentPostgresConfig.Host, strconv.Itoa(documentPostgresConfig.Port)), Path: "/" + documentPostgresConfig.Database, User: url.UserPassword(documentPostgresConfig.User, password)}
	values := url.Values{"sslmode": {documentPostgresConfig.SSLMode}}
	uRL.RawQuery = values.Encode()
	return uRL.String()
}

// Validate проверяет реквизиты подключения, размер пула, режим TLS и допустимые таймауты.
func (documentPostgresConfig DocumentPostgresConfig) Validate() error {
	if documentPostgresConfig.Host == "" || documentPostgresConfig.Port < 1 || documentPostgresConfig.Port > 65535 || documentPostgresConfig.Database == "" || documentPostgresConfig.User == "" || documentPostgresConfig.PoolMax < 1 || documentPostgresConfig.StatementTimeout.Time() < time.Millisecond || documentPostgresConfig.IdleTransactionTimeout.Time() < time.Millisecond || documentPostgresConfig.RollbackTimeout <= 0 {
		return fmt.Errorf("invalid PostgreSQL connection, pool or timeouts")
	}
	switch documentPostgresConfig.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return nil
	default:
		return fmt.Errorf("invalid PostgreSQL sslmode")
	}
}
