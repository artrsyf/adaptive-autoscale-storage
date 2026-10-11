package storage.controlplane.config

import java.nio.file.{Files, Path}

import org.yaml.snakeyaml.LoaderOptions
import org.yaml.snakeyaml.Yaml
import org.yaml.snakeyaml.constructor.SafeConstructor
import zio.{Task, ZIO}

import scala.concurrent.duration.{Duration, FiniteDuration}
import scala.jdk.CollectionConverters._

final case class ServerConfig(
    listenHost: String,
    port: Int,
    readHeaderTimeout: FiniteDuration,
    readTimeout: FiniteDuration,
    writeTimeout: FiniteDuration,
    idleTimeout: FiniteDuration,
    readyTimeout: FiniteDuration,
    shutdownTimeout: FiniteDuration
)

final case class ApiConfig(maxBodyBytes: Long, requestTimeout: FiniteDuration)

final case class PostgresConfig(
    host: String,
    port: Int,
    database: String,
    user: String,
    sslMode: String,
    poolMax: Int,
    statementTimeout: FiniteDuration,
    idleTransactionTimeout: FiniteDuration,
    rollbackTimeout: FiniteDuration
) {
  def jdbcUrl: String =
    s"jdbc:postgresql://$host:$port/$database?sslmode=$sslMode&ApplicationName=control-plane"
}

final case class ControlPlaneConfig(server: ServerConfig, api: ApiConfig, postgres: PostgresConfig) {
  def validate: Either[String, ControlPlaneConfig] = {
    val sslModes = Set("disable", "allow", "prefer", "require", "verify-ca", "verify-full")
    if (server.listenHost.trim.isEmpty || server.port < 1 || server.port > 65535)
      Left("invalid HTTP listener")
    else if (
      List(
        server.readHeaderTimeout,
        server.readTimeout,
        server.writeTimeout,
        server.idleTimeout,
        server.readyTimeout,
        server.shutdownTimeout,
        api.requestTimeout,
        postgres.statementTimeout,
        postgres.idleTransactionTimeout,
        postgres.rollbackTimeout
      ).exists(_.length <= 0)
    ) Left("timeouts must be positive")
    else if (server.writeTimeout <= api.requestTimeout) Left("write timeout must exceed request timeout")
    else if (api.maxBodyBytes < 1) Left("max body bytes must be positive")
    else if (
      postgres.host.trim.isEmpty || postgres.port < 1 || postgres.port > 65535 ||
      postgres.database.trim.isEmpty || postgres.user.trim.isEmpty || postgres.poolMax < 1
    ) Left("invalid PostgreSQL configuration")
    else if (!sslModes(postgres.sslMode)) Left("invalid PostgreSQL sslmode")
    else Right(this)
  }
}

object ControlPlaneConfig {
  def load(path: String): Task[ControlPlaneConfig] =
    for {
      content <- ZIO.attemptBlocking(Files.readString(Path.of(path)))
      config <- ZIO.fromEither(parse(content)).mapError(message => new IllegalArgumentException(message))
      valid <- ZIO.fromEither(config.validate).mapError(message => new IllegalArgumentException(message))
    } yield valid

  private def parse(content: String): Either[String, ControlPlaneConfig] =
    try {
      val loaderOptions = new LoaderOptions()
      loaderOptions.setAllowDuplicateKeys(false)
      val yaml = new Yaml(new SafeConstructor(loaderOptions))
      val root = asMap(yaml.load[Any](content), "root")
      val server = asMap(required(root, "server"), "server")
      val api = asMap(required(root, "api"), "api")
      val postgres = asMap(required(root, "postgres"), "postgres")
      ensureKeys(root, Set("server", "api", "postgres"), "root")
      ensureKeys(
        server,
        Set(
          "listen_host",
          "port",
          "read_header_timeout",
          "read_timeout",
          "write_timeout",
          "idle_timeout",
          "ready_timeout",
          "shutdown_timeout"
        ),
        "server"
      )
      ensureKeys(api, Set("max_body_bytes", "request_timeout"), "api")
      ensureKeys(
        postgres,
        Set(
          "host",
          "port",
          "database",
          "user",
          "sslmode",
          "pool_max",
          "statement_timeout",
          "idle_transaction_timeout",
          "rollback_timeout"
        ),
        "postgres"
      )
      Right(
        ControlPlaneConfig(
          ServerConfig(
            string(server, "listen_host"),
            integer(server, "port"),
            duration(server, "read_header_timeout"),
            duration(server, "read_timeout"),
            duration(server, "write_timeout"),
            duration(server, "idle_timeout"),
            duration(server, "ready_timeout"),
            duration(server, "shutdown_timeout")
          ),
          ApiConfig(long(api, "max_body_bytes"), duration(api, "request_timeout")),
          PostgresConfig(
            string(postgres, "host"),
            integer(postgres, "port"),
            string(postgres, "database"),
            string(postgres, "user"),
            string(postgres, "sslmode"),
            integer(postgres, "pool_max"),
            duration(postgres, "statement_timeout"),
            duration(postgres, "idle_transaction_timeout"),
            duration(postgres, "rollback_timeout")
          )
        )
      )
    } catch {
      case error: Exception => Left(s"invalid control-plane config: ${error.getMessage}")
    }

  private def asMap(value: Any, name: String): Map[String, Any] = value match {
    case mapping: java.util.Map[_, _] => mapping.asScala.iterator.map { case (key, item) => key.toString -> item }.toMap
    case _                            => throw new IllegalArgumentException(s"$name must be a mapping")
  }

  private def required(mapping: Map[String, Any], key: String): Any =
    mapping.getOrElse(key, throw new IllegalArgumentException(s"missing $key"))

  private def ensureKeys(mapping: Map[String, Any], allowed: Set[String], name: String): Unit = {
    val unknown = mapping.keySet -- allowed
    if (unknown.nonEmpty)
      throw new IllegalArgumentException(s"unknown $name keys: ${unknown.toList.sorted.mkString(", ")}")
  }

  private def string(mapping: Map[String, Any], key: String): String = required(mapping, key).toString

  private def integer(mapping: Map[String, Any], key: String): Int = required(mapping, key) match {
    case number: Number => number.intValue()
    case value          => value.toString.toInt
  }

  private def long(mapping: Map[String, Any], key: String): Long = required(mapping, key) match {
    case number: Number => number.longValue()
    case value          => value.toString.toLong
  }

  private def duration(mapping: Map[String, Any], key: String): FiniteDuration =
    Duration(required(mapping, key).toString) match {
      case finiteDuration: FiniteDuration => finiteDuration
      case _                              => throw new IllegalArgumentException(s"$key must be finite")
    }
}
