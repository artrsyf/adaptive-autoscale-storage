package storage.controlplane.assignment.repository.postgres

import doobie.hikari.HikariTransactor
import storage.controlplane.config.PostgresConfig
import zio.interop.catz._
import zio.{Scope, Task, TaskLayer, ZIO, ZLayer}

final case class PostgresPassword(value: String)

object PostgresPassword {
  val module: TaskLayer[PostgresPassword] = ZLayer.fromZIO(
    ZIO
      .fromOption(sys.env.get("POSTGRES_PASSWORD"))
      .mapError(_ => new IllegalArgumentException("POSTGRES_PASSWORD is required"))
      .map(PostgresPassword.apply)
  )
}

object PostgresDataSource {
  val layer: ZLayer[PostgresConfig & PostgresPassword, Throwable, HikariTransactor[Task]] = ZLayer.scoped {
    for {
      config <- ZIO.service[PostgresConfig]
      password <- ZIO.service[PostgresPassword]
      transactor <- make(config, password.value)
    } yield transactor
  }

  val module: TaskLayer[HikariTransactor[Task]] =
    ZLayer.make[HikariTransactor[Task]](PostgresConfig.module, PostgresPassword.module, layer)

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
