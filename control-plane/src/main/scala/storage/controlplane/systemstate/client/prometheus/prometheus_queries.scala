package storage.controlplane.systemstate.client.prometheus

import java.time.Duration

import storage.controlplane.systemstate.domain.SignalName

private[prometheus] object PrometheusQueries {
  def all(observationWindow: Duration): Map[SignalName, String] = {
    val window = s"${observationWindow.toSeconds}s"
    val requests = s"sum by(node)(rate(storage_requests_total{role=\"processing-unit\"}[$window]))"
    val requestDenominator = s"clamp_min($requests, 0.000001)"
    Map(
      SignalName.RequestsPerSecond -> requests,
      SignalName.HttpLatencyP95 -> histogram(0.95, "storage_request_duration_seconds_bucket", window),
      SignalName.HttpLatencyP99 -> histogram(0.99, "storage_request_duration_seconds_bucket", window),
      SignalName.ServerErrorRate ->
        s"(sum by(node)(rate(storage_requests_total{role=\"processing-unit\",status=~\"5..\"}[$window])) / on(node) $requestDenominator) or on(node) (0 * $requests)",
      SignalName.OverloadRate ->
        s"(sum by(node)(rate(storage_requests_total{role=\"processing-unit\",status=\"429\"}[$window])) / on(node) $requestDenominator) or on(node) (0 * $requests)",
      SignalName.ActiveRequests -> "max by(node)(storage_active_requests{role=\"processing-unit\"})",
      SignalName.PoolAcquireP95 -> histogram(0.95, "storage_pool_acquire_seconds_bucket", window),
      SignalName.DatabaseOperationP95 -> histogram(0.95, "storage_db_operation_seconds_bucket", window),
      SignalName.CpuUtilization -> resourceUtilization(
        s"sum by(container_label_com_docker_compose_service)(rate(container_cpu_usage_seconds_total{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\",image!=\"\"}[$window]))",
        "max by(container_label_com_docker_compose_service)(container_spec_cpu_quota{container_label_com_docker_compose_project=\"autoscale-storage\"} / container_spec_cpu_period{container_label_com_docker_compose_project=\"autoscale-storage\"})"
      ),
      SignalName.MemoryUtilization -> resourceUtilization(
        "max by(container_label_com_docker_compose_service)(container_memory_working_set_bytes{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\",image!=\"\"})",
        "max by(container_label_com_docker_compose_service)(container_spec_memory_limit_bytes{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\"})"
      )
    )
  }

  private def histogram(quantile: Double, metric: String, window: String): String =
    s"histogram_quantile($quantile,sum by(le,node)(rate($metric{role=\"processing-unit\"}[$window])))"

  private def resourceUtilization(numerator: String, denominator: String): String =
    s"label_replace(($numerator) / on(container_label_com_docker_compose_service) ($denominator), \"node\", \"$$1\", \"container_label_com_docker_compose_service\", \"(.*)\")"
}
