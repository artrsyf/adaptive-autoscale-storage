package storage.controlplane.systemstate.service

import java.time.{Duration, Instant}

import storage.controlplane.systemstate.domain.{MetricUnit, SignalName}
import zio.IO

enum MetricsSourceError {
  case Transport(message: String)
  case Protocol(message: String)
  case InvalidData(message: String)
}

final case class MetricSeries(
    signal: SignalName,
    nodeId: String,
    value: Double,
    unit: MetricUnit,
    sampledAt: Instant
)

trait MetricsSource {
  def collect(
      expectedNodes: Set[String],
      observationWindow: Duration,
      evaluatedAt: Instant
  ): IO[MetricsSourceError, List[MetricSeries]]
}
