package storage.controlplane.systemstate.client.prometheus

import java.net.URLEncoder
import java.nio.charset.StandardCharsets
import java.time.{Duration, Instant}

import storage.controlplane.config.MetricsConfig
import storage.controlplane.systemstate.domain.SignalName
import storage.controlplane.systemstate.service.{MetricSeries, MetricsSource, MetricsSourceError}
import zio.http.{Client, Request, URL}
import zio.json._
import zio.{TaskLayer, URLayer, ZIO, ZLayer}

import scala.concurrent.duration.FiniteDuration

final class PrometheusMetricsSource(config: MetricsConfig, client: Client) extends MetricsSource {
  override def collect(
      expectedNodes: Set[String],
      observationWindow: Duration,
      evaluatedAt: Instant
  ): ZIO[Any, MetricsSourceError, List[MetricSeries]] =
    ZIO
      .foreachPar(PrometheusQueries.all(observationWindow).toList) { case (signal, query) =>
        execute(signal, query, evaluatedAt)
      }
      .map(_.flatten)

  private def execute(
      signal: SignalName,
      query: String,
      evaluatedAt: Instant
  ): ZIO[Any, MetricsSourceError, List[MetricSeries]] = {
    val encodedQuery = URLEncoder.encode(query, StandardCharsets.UTF_8)
    val separator = if (config.prometheusUrl.endsWith("/")) "" else "/"
    val address =
      s"${config.prometheusUrl}${separator}api/v1/query?query=$encodedQuery&time=${evaluatedAt.getEpochSecond}"
    def request = for {
      url <- ZIO.fromEither(URL.decode(address)).mapError(error => MetricsSourceError.InvalidData(error.getMessage))
      response <- client
        .batched(Request.get(url))
        .mapError(error => MetricsSourceError.Transport(error.getMessage))
        .timeoutFail(MetricsSourceError.Transport("Prometheus query timed out"))(
          zio.Duration.fromMillis(config.queryTimeout.toMillis)
        )
      body <- response.body.asString.mapError(error => MetricsSourceError.Transport(error.getMessage))
      _ <- ZIO
        .fail(MetricsSourceError.Protocol(s"Prometheus returned HTTP ${response.status.code}"))
        .unless(response.status.isSuccess)
      samples <- decode(signal, body)
    } yield samples

    def retry(remainingAttempts: Int, delay: FiniteDuration): ZIO[Any, MetricsSourceError, List[MetricSeries]] =
      request.catchAll { error =>
        if (remainingAttempts <= 1) ZIO.fail(error)
        else {
          val nextDelay = (delay * 2).min(config.retry.maxBackoff)
          ZIO.sleep(zio.Duration.fromMillis(delay.toMillis)) *> retry(remainingAttempts - 1, nextDelay)
        }
      }

    retry(config.retry.attempts, config.retry.initialBackoff)
  }

  private[prometheus] def decode(
      signal: SignalName,
      body: String
  ): ZIO[Any, MetricsSourceError, List[MetricSeries]] =
    for {
      response <- ZIO.fromEither(body.fromJson[PrometheusResponse]).mapError(MetricsSourceError.InvalidData.apply)
      data <- ZIO
        .fromOption(response.data)
        .orElseFail(MetricsSourceError.Protocol(response.error.getOrElse("Prometheus response has no data")))
      _ <- ZIO
        .fail(MetricsSourceError.Protocol("Prometheus response is not a vector"))
        .unless(response.status == "success" && data.resultType == "vector")
      samples <- ZIO.foreach(data.result) { sample =>
        val (timestamp, encodedValue) = sample.value
        for {
          nodeId <- ZIO
            .fromOption(sample.metric.get("node"))
            .orElseFail(MetricsSourceError.InvalidData(s"${signal.wireName} sample has no node label"))
          value <- ZIO
            .attempt(encodedValue.toDouble)
            .mapError(_ => MetricsSourceError.InvalidData(s"${signal.wireName} sample value is not numeric"))
          _ <- ZIO
            .fail(MetricsSourceError.InvalidData(s"${signal.wireName} sample is negative or non-finite"))
            .when(!value.isFinite || value < 0)
        } yield MetricSeries(
          signal,
          nodeId,
          value,
          signal.unit,
          Instant.ofEpochMilli((timestamp * 1000).toLong)
        )
      }
    } yield samples
}

object PrometheusMetricsSource {
  val layer: URLayer[MetricsConfig & Client, MetricsSource] = ZLayer.derive[PrometheusMetricsSource]
  val module: TaskLayer[MetricsSource] =
    ZLayer.make[MetricsSource](MetricsConfig.module, Client.default, layer)
}
