package storage.controlplane.config

import java.nio.file.{Files, Path}

import org.yaml.snakeyaml.LoaderOptions
import org.yaml.snakeyaml.Yaml
import org.yaml.snakeyaml.constructor.SafeConstructor
import zio.{Task, TaskLayer, URLayer, ZIO, ZLayer}

import scala.concurrent.duration.{Duration, FiniteDuration}
import scala.jdk.CollectionConverters._
import storage.controlplane.scaling.domain.{ScalingConfig, ThresholdBand}
import storage.controlplane.systemstate.domain.SignalName

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
object ServerConfig {
  val layer: URLayer[ControlPlaneConfig, ServerConfig] =
    ZLayer.fromFunction((config: ControlPlaneConfig) => config.server)
  val module: TaskLayer[ServerConfig] = ZLayer.make[ServerConfig](ControlPlaneConfig.module, layer)
}

final case class ApiConfig(maxBodyBytes: Long, requestTimeout: FiniteDuration)
object ApiConfig {
  val layer: URLayer[ControlPlaneConfig, ApiConfig] = ZLayer.fromFunction((config: ControlPlaneConfig) => config.api)
  val module: TaskLayer[ApiConfig] = ZLayer.make[ApiConfig](ControlPlaneConfig.module, layer)
}

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
object PostgresConfig {
  val layer: URLayer[ControlPlaneConfig, PostgresConfig] =
    ZLayer.fromFunction((config: ControlPlaneConfig) => config.postgres)
  val module: TaskLayer[PostgresConfig] = ZLayer.make[PostgresConfig](ControlPlaneConfig.module, layer)
}

final case class MetricsRetryConfig(attempts: Int, initialBackoff: FiniteDuration, maxBackoff: FiniteDuration)

final case class MetricsConfig(
    prometheusUrl: String,
    pollInterval: FiniteDuration,
    queryTimeout: FiniteDuration,
    observationWindow: FiniteDuration,
    maxSampleAge: FiniteDuration,
    retry: MetricsRetryConfig
)
object MetricsConfig {
  val layer: URLayer[ControlPlaneConfig, MetricsConfig] =
    ZLayer.fromFunction((config: ControlPlaneConfig) => config.metrics)
  val module: TaskLayer[MetricsConfig] = ZLayer.make[MetricsConfig](ControlPlaneConfig.module, layer)
}

final case class ControlPlaneConfig(
    server: ServerConfig,
    api: ApiConfig,
    postgres: PostgresConfig,
    metrics: MetricsConfig,
    scaling: ScalingConfig
) {
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
        postgres.rollbackTimeout,
        metrics.pollInterval,
        metrics.queryTimeout,
        metrics.observationWindow,
        metrics.maxSampleAge,
        metrics.retry.initialBackoff,
        metrics.retry.maxBackoff
      ).exists(_.length <= 0)
    ) Left("timeouts must be positive")
    else if (server.writeTimeout <= api.requestTimeout) Left("write timeout must exceed request timeout")
    else if (api.maxBodyBytes < 1) Left("max body bytes must be positive")
    else if (
      postgres.host.trim.isEmpty || postgres.port < 1 || postgres.port > 65535 ||
      postgres.database.trim.isEmpty || postgres.user.trim.isEmpty || postgres.poolMax < 1
    ) Left("invalid PostgreSQL configuration")
    else if (!sslModes(postgres.sslMode)) Left("invalid PostgreSQL sslmode")
    else if (!validHttpOrigin(metrics.prometheusUrl)) Left("invalid Prometheus URL")
    else if (metrics.retry.attempts < 1 || metrics.retry.maxBackoff < metrics.retry.initialBackoff)
      Left("invalid metrics retry configuration")
    else if (metrics.maxSampleAge < metrics.pollInterval) Left("max sample age must cover at least one poll interval")
    else scaling.validate.map(_ => this)
  }

  private def validHttpOrigin(value: String): Boolean =
    try {
      val uri = java.net.URI.create(value)
      Set("http", "https").contains(
        uri.getScheme
      ) && uri.getHost != null && uri.getQuery == null && uri.getFragment == null
    } catch {
      case _: IllegalArgumentException => false
    }
}

object ControlPlaneConfig {
  val module: TaskLayer[ControlPlaneConfig] = ZLayer.fromZIO(load("config.yaml"))

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
      val metrics = asMap(required(root, "metrics"), "metrics")
      val retry = asMap(required(metrics, "retry"), "metrics.retry")
      val scaling = asMap(required(root, "scaling"), "scaling")
      val thresholds = asMap(required(scaling, "thresholds"), "scaling.thresholds")
      ensureKeys(root, Set("server", "api", "postgres", "metrics", "scaling"), "root")
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
      ensureKeys(
        metrics,
        Set("prometheus_url", "poll_interval", "query_timeout", "observation_window", "max_sample_age", "retry"),
        "metrics"
      )
      ensureKeys(retry, Set("attempts", "initial_backoff", "max_backoff"), "metrics.retry")
      ensureKeys(
        scaling,
        Set("min_replicas", "max_replicas", "step", "confirmation_windows", "cooldown", "thresholds"),
        "scaling"
      )
      ensureKeys(thresholds, SignalName.policySignals.map(_.wireName), "scaling.thresholds")
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
          ),
          MetricsConfig(
            string(metrics, "prometheus_url"),
            duration(metrics, "poll_interval"),
            duration(metrics, "query_timeout"),
            duration(metrics, "observation_window"),
            duration(metrics, "max_sample_age"),
            MetricsRetryConfig(
              integer(retry, "attempts"),
              duration(retry, "initial_backoff"),
              duration(retry, "max_backoff")
            )
          ),
          ScalingConfig(
            integer(scaling, "min_replicas"),
            integer(scaling, "max_replicas"),
            integer(scaling, "step"),
            integer(scaling, "confirmation_windows"),
            java.time.Duration.ofMillis(duration(scaling, "cooldown").toMillis),
            SignalName.policySignals.map { signal =>
              val threshold = asMap(required(thresholds, signal.wireName), s"threshold ${signal.wireName}")
              ensureKeys(threshold, Set("lower", "upper"), s"threshold ${signal.wireName}")
              signal -> ThresholdBand(decimal(threshold, "lower"), decimal(threshold, "upper"))
            }.toMap
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

  private def decimal(mapping: Map[String, Any], key: String): Double = required(mapping, key) match {
    case number: Number => number.doubleValue()
    case value          => value.toString.toDouble
  }

  private def duration(mapping: Map[String, Any], key: String): FiniteDuration =
    Duration(required(mapping, key).toString) match {
      case finiteDuration: FiniteDuration => finiteDuration
      case _                              => throw new IllegalArgumentException(s"$key must be finite")
    }
}
