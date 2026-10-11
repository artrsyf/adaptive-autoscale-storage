package storage.controlplane.systemstate.service

import java.time.{Duration, Instant}

import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.config.{MetricsConfig, MetricsRetryConfig}
import storage.controlplane.systemstate.domain._
import zio.test._
import zio.{IO, Ref, ZIO}

import scala.concurrent.duration._

object StateAggregatorSpec extends ZIOSpecDefault {
  private val observedAt = Instant.parse("2026-10-11T12:00:00Z")
  private val assignment = AssignmentSnapshot(
    7,
    2,
    List(Node("one", "http://one:8080"), Node("two", "http://two:8080")),
    List(PartitionOwner(0, "one"), PartitionOwner(1, "two"))
  )
  private val config = MetricsConfig(
    "http://prometheus:9090",
    5.seconds,
    2.seconds,
    30.seconds,
    15.seconds,
    MetricsRetryConfig(2, 250.millis, 1.second)
  )

  override def spec = suite("StateAggregator")(
    test("aggregates a complete topology and keeps total RPS separately") {
      for {
        assignmentRef <- Ref.make(assignment)
        aggregator = new StateAggregator(source(assignmentRef), metrics(completeSeries), config)
        state <- aggregator.collectOnce(observedAt)
      } yield assertTrue(
        state.dataQuality.complete,
        state.cluster.signals(SignalName.RequestsPerSecond).value == 2.0,
        state.cluster.totalRequestsPerSecond.exists(_.value == 3.0),
        state.database.nonEmpty
      )
    },
    test("marks missing, stale, duplicate and unknown-node series") {
      for {
        assignmentRef <- Ref.make(assignment)
        base = completeSeries.filterNot(metric =>
          metric.nodeId == "two" && metric.signal == SignalName.MemoryUtilization
        )
        stale = base.map { metric =>
          if (metric.nodeId == "one" && metric.signal == SignalName.CpuUtilization)
            metric.copy(sampledAt = observedAt.minusSeconds(30))
          else metric
        }
        duplicate = stale :+ stale.head
        withUnknown = duplicate :+ stale.head.copy(nodeId = "other")
        aggregator = new StateAggregator(source(assignmentRef), metrics(withUnknown), config)
        state <- aggregator.collectOnce(observedAt)
      } yield assertTrue(
        !state.dataQuality.complete,
        state.dataQuality.missingSignals.contains(SignalReference(SignalName.MemoryUtilization, Some("two"))),
        state.dataQuality.staleSignals.contains(SignalReference(SignalName.CpuUtilization, Some("one"))),
        state.dataQuality.duplicateSignals.nonEmpty,
        state.dataQuality.unknownNodes == Set("other")
      )
    },
    test("publishes unavailable data quality instead of failing on Prometheus") {
      for {
        assignmentRef <- Ref.make(assignment)
        failing = new MetricsSource {
          override def collect(expectedNodes: Set[String], observationWindow: Duration, evaluatedAt: Instant) =
            ZIO.fail(MetricsSourceError.Transport("offline"))
        }
        aggregator = new StateAggregator(source(assignmentRef), failing, config)
        state <- aggregator.collectOnce(observedAt)
      } yield assertTrue(
        state.dataQuality.sourceStatus == MetricsSourceStatus.Unavailable,
        !state.dataQuality.complete,
        state.dataQuality.missingSignals.size == assignment.nodes.size * SignalName.values.size
      )
    },
    test("discards an observation when assignment changes during collection") {
      for {
        calls <- Ref.make(0)
        changingSource = new AssignmentSnapshotSource {
          override def current: IO[AssignmentError, AssignmentSnapshot] =
            calls.getAndUpdate(_ + 1).map(call => if (call == 0) assignment else assignment.copy(epoch = 8))
        }
        aggregator = new StateAggregator(changingSource, metrics(completeSeries), config)
        result <- aggregator.collectOnce(observedAt).either
      } yield assertTrue(result == Left(StateAggregationError.AssignmentChanged))
    }
  )

  private def source(state: Ref[AssignmentSnapshot]): AssignmentSnapshotSource = new AssignmentSnapshotSource {
    override def current: IO[AssignmentError, AssignmentSnapshot] = state.get
  }

  private def metrics(series: List[MetricSeries]): MetricsSource = new MetricsSource {
    override def collect(expectedNodes: Set[String], observationWindow: Duration, evaluatedAt: Instant) =
      ZIO.succeed(series)
  }

  private val completeSeries = for {
    node <- List("one", "two")
    signal <- SignalName.values.toList
  } yield MetricSeries(
    signal,
    node,
    if (node == "one") 1.0 else 2.0,
    signal.unit,
    observedAt.minusSeconds(1)
  )
}
