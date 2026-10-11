package storage.controlplane.scaling.service

import java.time.{Duration, Instant}

import storage.controlplane.config.{MetricsConfig, MetricsRetryConfig}
import storage.controlplane.scaling.domain._
import storage.controlplane.systemstate.domain._
import storage.controlplane.systemstate.service.{StateAggregationError, SystemStateCollector}
import zio.test._
import zio.{Ref, ZIO}

import scala.concurrent.duration._

object ControllerServiceSpec extends ZIOSpecDefault {
  private val observedAt = Instant.parse("2026-10-11T12:00:00Z")
  private val thresholds = SignalName.policySignals.map(signal => signal -> ThresholdBand(0.25, 0.75)).toMap
  private val scalingConfig = ScalingConfig(1, 5, 1, 2, Duration.ofSeconds(30), thresholds)
  private val metricsConfig = MetricsConfig(
    "http://prometheus:9090",
    5.seconds,
    2.seconds,
    30.seconds,
    15.seconds,
    MetricsRetryConfig(2, 250.millis, 1.second)
  )

  override def spec = suite("ControllerService")(
    test("publishes healthy decisions and degraded incomplete states") {
      for {
        resultRef <- Ref.make[Either[StateAggregationError, SystemState]](Right(state(complete = true)))
        collector = collectorFrom(resultRef)
        snapshotRef <- Ref.make(ControllerSnapshot.collecting)
        controller = ControllerService(collector, scalingConfig, metricsConfig, snapshotRef)
        _ <- controller.runOnce
        healthy <- controller.snapshot
        _ <- resultRef.set(Right(state(complete = false)))
        _ <- controller.runOnce
        degraded <- controller.snapshot
      } yield assertTrue(
        healthy.status == ControllerStatus.Healthy,
        healthy.decision.exists(_.kind == ScalingDecisionKind.Maintain),
        healthy.completeCycles == 1,
        degraded.status == ControllerStatus.Degraded,
        degraded.decision.exists(_.kind == ScalingDecisionKind.InsufficientData),
        degraded.incompleteCycles == 1
      )
    },
    test("converts collection failures to a non-actionable decision") {
      for {
        resultRef <- Ref.make[Either[StateAggregationError, SystemState]](Right(state(complete = true)))
        snapshotRef <- Ref.make(ControllerSnapshot.collecting)
        controller = ControllerService(collectorFrom(resultRef), scalingConfig, metricsConfig, snapshotRef)
        _ <- controller.runOnce
        _ <- resultRef.set(Left(StateAggregationError.AssignmentChanged))
        _ <- controller.runOnce
        snapshot <- controller.snapshot
      } yield assertTrue(
        snapshot.status == ControllerStatus.Degraded,
        snapshot.decision.exists(_.kind == ScalingDecisionKind.InsufficientData),
        snapshot.decision.exists(_.blockedBy == List("assignment_unavailable_or_changed")),
        snapshot.failedCycles == 1
      )
    },
    test("does not overlap polling cycles") {
      for {
        active <- Ref.make(0)
        maximumActive <- Ref.make(0)
        calls <- Ref.make(0)
        collector = new SystemStateCollector {
          override def collectOnce(evaluatedAt: Instant) =
            ZIO.acquireReleaseWith(
              active.updateAndGet(_ + 1).tap(current => maximumActive.update(_.max(current)))
            )(_ => active.update(_ - 1))(_ =>
              ZIO.sleep(zio.Duration.fromSeconds(2)) *> calls.update(_ + 1).as(state(complete = true))
            )
        }
        snapshotRef <- Ref.make(ControllerSnapshot.collecting)
        controller = ControllerService(collector, scalingConfig, metricsConfig, snapshotRef)
        fiber <- controller.run.fork
        _ <- TestClock.adjust(zio.Duration.fromSeconds(30))
        _ <- ZIO.yieldNow
        _ <- fiber.interrupt
        observedMaximum <- maximumActive.get
        observedCalls <- calls.get
      } yield assertTrue(observedMaximum == 1, observedCalls >= 2)
    }
  )

  private def collectorFrom(resultRef: Ref[Either[StateAggregationError, SystemState]]): SystemStateCollector =
    new SystemStateCollector {
      override def collectOnce(evaluatedAt: Instant) = resultRef.get.flatMap(ZIO.fromEither(_))
    }

  private def state(complete: Boolean): SystemState = {
    val values = SignalName.policySignals.map { signal =>
      signal -> MetricValue.make(0.5, signal.unit, observedAt).toOption.get
    }.toMap
    val observed = SignalName.values.map { signal =>
      val metric = values.get(signal)
      signal -> ObservedMetric(
        metric,
        DataFreshness(
          if (metric.isDefined) FreshnessKind.Fresh else FreshnessKind.Missing,
          metric.map(_ => Duration.ZERO)
        )
      )
    }.toMap
    val missing =
      if (complete) Set.empty[SignalReference]
      else Set(SignalReference(SignalName.CpuUtilization, Some("one")))
    SystemState(
      observedAt,
      MetricWindow(observedAt.minusSeconds(30), observedAt),
      7,
      Set("one", "two"),
      List(ProcessingUnitState("one", observed), ProcessingUnitState("two", observed)),
      Some(DatabaseState(values(SignalName.PoolAcquireP95), values(SignalName.DatabaseOperationP95))),
      ClusterState(
        if (complete) values else values - SignalName.CpuUtilization,
        values.get(SignalName.RequestsPerSecond)
      ),
      DataQuality(complete, MetricsSourceStatus.Available, missing, Set.empty, Set.empty, Set.empty)
    )
  }
}
