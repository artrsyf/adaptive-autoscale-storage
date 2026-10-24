package storage.controlplane.systemstate.client.prometheus

import java.time.Duration

import storage.controlplane.systemstate.domain.SignalName

private[prometheus] object PrometheusQueries {
  def window(observationWindow: Duration): String = s"${observationWindow.toSeconds}s"

  def common(window: String): Map[SignalName, String] = {
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
      SignalName.DatabaseOperationP95 -> histogram(0.95, "storage_db_operation_seconds_bucket", window)
    )
  }

  private def histogram(quantile: Double, metric: String, window: String): String =
    s"histogram_quantile($quantile,sum by(le,node)(rate($metric{role=\"processing-unit\"}[$window])))"
}
