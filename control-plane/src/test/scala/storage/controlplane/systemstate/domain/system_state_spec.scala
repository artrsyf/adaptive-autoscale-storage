package storage.controlplane.systemstate.domain

import java.time.Instant

import zio.test._

object SystemStateSpec extends ZIOSpecDefault {
  override def spec = suite("SystemState")(
    test("rejects topology and unit mismatches") {
      val observedAt = Instant.parse("2026-10-11T12:00:00Z")
      val state = SystemState(
        observedAt,
        MetricWindow(observedAt.minusSeconds(30), observedAt),
        1,
        Set("one", "two"),
        List(ProcessingUnitState("one", Map.empty)),
        None,
        ClusterState(Map.empty, None),
        DataQuality(false, MetricsSourceStatus.Available, Set.empty, Set.empty, Set.empty, Set.empty)
      )
      assertTrue(state.validate.isLeft)
    },
    test("validates metric units") {
      val observedAt = Instant.parse("2026-10-11T12:00:00Z")
      val wrongMetric = MetricValue.make(1, MetricUnit.Seconds, observedAt).toOption.get
      val state = SystemState(
        observedAt,
        MetricWindow(observedAt.minusSeconds(30), observedAt),
        1,
        Set("one"),
        List(ProcessingUnitState("one", Map.empty)),
        None,
        ClusterState(Map(SignalName.CpuUtilization -> wrongMetric), None),
        DataQuality(true, MetricsSourceStatus.Available, Set.empty, Set.empty, Set.empty, Set.empty)
      )
      assertTrue(state.validate == Left("cluster signal unit mismatch"))
    }
  )
}
