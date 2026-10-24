package storage.controlplane.systemstate.client.prometheus

import java.time.Duration

import storage.controlplane.systemstate.domain.SignalName

trait PrometheusQuerySet {
  final def all(observationWindow: Duration): Map[SignalName, String] = {
    val window = PrometheusQueries.window(observationWindow)
    PrometheusQueries.common(window) ++ Map(
      SignalName.CpuUtilization -> cpuUtilization(window),
      SignalName.MemoryUtilization -> memoryUtilization
    )
  }

  protected def cpuUtilization(window: String): String
  protected def memoryUtilization: String
}
