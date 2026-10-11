package storage.controlplane.systemstate.domain

import java.time.{Duration, Instant}

enum MetricUnit(val wireName: String) {
  case RequestsPerSecond extends MetricUnit("requests_per_second")
  case Ratio extends MetricUnit("ratio")
  case Seconds extends MetricUnit("seconds")
  case Requests extends MetricUnit("requests")
}

final case class MetricValue private (value: Double, unit: MetricUnit, sampledAt: Instant)

object MetricValue {
  def make(value: Double, unit: MetricUnit, sampledAt: Instant): Either[String, MetricValue] =
    if (!value.isFinite || value < 0) Left("metric value must be finite and non-negative")
    else Right(MetricValue(value, unit, sampledAt))
}

final case class MetricWindow(startedAt: Instant, endedAt: Instant) {
  def validate: Either[String, MetricWindow] =
    if (startedAt.isAfter(endedAt)) Left("metric window start must not be after its end") else Right(this)
}

enum FreshnessKind(val wireName: String) {
  case Fresh extends FreshnessKind("fresh")
  case Stale extends FreshnessKind("stale")
  case Missing extends FreshnessKind("missing")
  case Invalid extends FreshnessKind("invalid")
}

final case class DataFreshness(kind: FreshnessKind, age: Option[Duration])

final case class ObservedMetric(value: Option[MetricValue], freshness: DataFreshness)
