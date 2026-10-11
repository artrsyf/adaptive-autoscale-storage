package storage.controlplane.app

import java.time.{Duration, Instant}

import storage.controlplane.scaling.domain._
import storage.controlplane.systemstate.domain._
import zio.test._

object ControllerMetricsSpec extends ZIOSpecDefault {
  private val now = Instant.parse("2026-10-11T12:00:00Z")
  private val thresholds = SignalName.policySignals.map(signal => signal -> ThresholdBand(0.25, 0.75)).toMap
  private val config = ScalingConfig(1, 5, 1, 3, Duration.ofSeconds(30), thresholds)

  override def spec = suite("ControllerMetrics")(
    test("does not invent values while collecting") {
      val metrics = ControllerMetrics.render(ControllerSnapshot.collecting, config, now, 7)
      assertTrue(
        metrics.contains("storage_control_controller_status{status=\"collecting\"} 1.0"),
        !metrics.contains("storage_control_signal_value{"),
        !metrics.contains("storage_control_current_replicas ")
      )
    },
    test("publishes evaluated values, thresholds and pressure") {
      val metric = MetricValue.make(0.5, MetricUnit.Ratio, now.minusSeconds(1)).toOption.get
      val state = SystemState(
        now,
        MetricWindow(now.minusSeconds(30), now),
        7,
        Set("one"),
        List(
          ProcessingUnitState(
            "one",
            SignalName.values
              .map(signal => signal -> ObservedMetric(None, DataFreshness(FreshnessKind.Missing, None)))
              .toMap
          )
        ),
        None,
        ClusterState(Map(SignalName.CpuUtilization -> metric), None),
        DataQuality(false, MetricsSourceStatus.Available, Set.empty, Set.empty, Set.empty, Set.empty)
      )
      val snapshot = ControllerSnapshot.collecting.copy(status = ControllerStatus.Degraded, systemState = Some(state))
      val metrics = ControllerMetrics.render(snapshot, config, now, 7)
      assertTrue(
        metrics.contains("storage_control_signal_value{signal=\"cpu_utilization\",unit=\"ratio\"} 0.5"),
        metrics.contains("storage_control_signal_upper_threshold{signal=\"cpu_utilization\",unit=\"ratio\"} 0.75"),
        metrics.contains("storage_control_signal_pressure_ratio{signal=\"cpu_utilization\"} 0.666")
      )
    }
  )
}
