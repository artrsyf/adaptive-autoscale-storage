package storage.controlplane.assignment.repository.postgres

import doobie.hikari.HikariTransactor
import storage.controlplane.config.PostgresConfig
import zio.interop.catz._
import zio.{Scope, Task, ZIO}

object PostgresDataSource {
  def make(config: PostgresConfig, password: String): ZIO[Scope, Throwable, HikariTransactor[Task]] =
    for {
      connectExecutionContext <- ZIO.executor.map(_.asExecutionContext)
      transactor <- HikariTransactor
        .newHikariTransactor[Task](
          driverClassName = "org.postgresql.Driver",
          url = config.jdbcUrl,
          user = config.user,
          pass = password,
          connectEC = connectExecutionContext,
          logHandler = None
        )
        .toScopedZIO
      _ <- ZIO.attempt {
        transactor.kernel.setMaximumPoolSize(config.poolMax)
        transactor.kernel.setMinimumIdle(0)
        transactor.kernel.setConnectionInitSql(
          s"SET statement_timeout = ${config.statementTimeout.toMillis}; " +
            s"SET idle_in_transaction_session_timeout = ${config.idleTransactionTimeout.toMillis}"
        )
      }
    } yield transactor
}
