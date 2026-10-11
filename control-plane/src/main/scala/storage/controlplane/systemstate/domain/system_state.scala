package storage.controlplane.systemstate.domain

import java.time.Instant

enum SignalName(val wireName: String, val unit: MetricUnit) {
  case RequestsPerSecond extends SignalName("requests_per_second", MetricUnit.RequestsPerSecond)
  case HttpLatencyP95 extends SignalName("http_latency_p95", MetricUnit.Seconds)
  case HttpLatencyP99 extends SignalName("http_latency_p99", MetricUnit.Seconds)
  case ServerErrorRate extends SignalName("server_error_rate", MetricUnit.Ratio)
  case OverloadRate extends SignalName("overload_rate", MetricUnit.Ratio)
  case ActiveRequests extends SignalName("active_requests", MetricUnit.Requests)
  case PoolAcquireP95 extends SignalName("pool_acquire_p95", MetricUnit.Seconds)
  case DatabaseOperationP95 extends SignalName("database_operation_p95", MetricUnit.Seconds)
  case CpuUtilization extends SignalName("cpu_utilization", MetricUnit.Ratio)
  case MemoryUtilization extends SignalName("memory_utilization", MetricUnit.Ratio)
}

object SignalName {
  val policySignals: Set[SignalName] = Set(
    RequestsPerSecond,
    HttpLatencyP99,
    ServerErrorRate,
    OverloadRate,
    ActiveRequests,
    PoolAcquireP95,
    DatabaseOperationP95,
    CpuUtilization,
    MemoryUtilization
  )
}

final case class ProcessingUnitState(nodeId: String, signals: Map[SignalName, ObservedMetric])

final case class DatabaseState(poolAcquireP95: MetricValue, operationP95: MetricValue)

final case class ClusterState(signals: Map[SignalName, MetricValue], totalRequestsPerSecond: Option[MetricValue])

final case class SignalReference(signal: SignalName, nodeId: Option[String])

enum MetricsSourceStatus(val wireName: String) {
  case Available extends MetricsSourceStatus("available")
  case Unavailable extends MetricsSourceStatus("unavailable")
}

final case class DataQuality(
    complete: Boolean,
    sourceStatus: MetricsSourceStatus,
    missingSignals: Set[SignalReference],
    staleSignals: Set[SignalReference],
    duplicateSignals: Set[SignalReference],
    unknownNodes: Set[String]
)

final case class SystemState(
    observedAt: Instant,
    window: MetricWindow,
    assignmentEpoch: Long,
    expectedNodes: Set[String],
    processingUnits: List[ProcessingUnitState],
    database: Option[DatabaseState],
    cluster: ClusterState,
    dataQuality: DataQuality
) {
  def validate: Either[String, SystemState] =
    if (assignmentEpoch <= 0 || expectedNodes.isEmpty) Left("system state requires a positive epoch and active nodes")
    else if (
      processingUnits
        .map(_.nodeId)
        .toSet != expectedNodes || processingUnits.map(_.nodeId).distinct.size != processingUnits.size
    )
      Left("processing unit states must match the active assignment exactly")
    else if (cluster.signals.exists { case (signal, metric) => signal.unit != metric.unit })
      Left("cluster signal unit mismatch")
    else window.validate.map(_ => this)
}
