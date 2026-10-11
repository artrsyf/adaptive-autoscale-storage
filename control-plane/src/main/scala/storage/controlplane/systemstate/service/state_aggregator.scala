package storage.controlplane.systemstate.service

import java.time.{Duration, Instant}

import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot}
import storage.controlplane.config.MetricsConfig
import storage.controlplane.systemstate.client.prometheus.PrometheusMetricsSource
import storage.controlplane.systemstate.domain._
import zio.{IO, TaskLayer, URLayer, ZIO, ZLayer}

enum StateAggregationError {
  case AssignmentUnavailable(cause: AssignmentError)
  case AssignmentChanged
}

trait SystemStateCollector {
  def collectOnce(observedAt: Instant): IO[StateAggregationError, SystemState]
}

final class StateAggregator(
    assignmentSource: AssignmentSnapshotSource,
    metricsSource: MetricsSource,
    config: MetricsConfig
) extends SystemStateCollector {
  override def collectOnce(observedAt: Instant): IO[StateAggregationError, SystemState] =
    for {
      initialAssignment <- assignmentSource.current.mapError(StateAggregationError.AssignmentUnavailable.apply)
      collected <- metricsSource
        .collect(
          initialAssignment.nodes.map(_.id).toSet,
          Duration.ofMillis(config.observationWindow.toMillis),
          observedAt
        )
        .either
      systemState = collected match {
        case Right(series) => aggregate(initialAssignment, series, observedAt, MetricsSourceStatus.Available)
        case Left(_)       => aggregate(initialAssignment, Nil, observedAt, MetricsSourceStatus.Unavailable)
      }
      currentAssignment <- assignmentSource.current.mapError(StateAggregationError.AssignmentUnavailable.apply)
      _ <- ZIO
        .fail(StateAggregationError.AssignmentChanged)
        .unless(
          currentAssignment.epoch == initialAssignment.epoch &&
            currentAssignment.nodes.map(_.id).toSet == initialAssignment.nodes.map(_.id).toSet
        )
    } yield systemState

  private[service] def aggregate(
      assignment: AssignmentSnapshot,
      series: List[MetricSeries],
      observedAt: Instant,
      sourceStatus: MetricsSourceStatus
  ): SystemState = {
    val expectedNodes = assignment.nodes.map(_.id).toSet
    val unknownNodes = series.iterator.map(_.nodeId).filterNot(expectedNodes).toSet
    val expectedSeries = series.filter(metric => expectedNodes(metric.nodeId))
    val grouped = expectedSeries.groupBy(metric => metric.nodeId -> metric.signal)
    val maxSampleAge = Duration.ofMillis(config.maxSampleAge.toMillis)
    var missingSignals = Set.empty[SignalReference]
    var staleSignals = Set.empty[SignalReference]
    var duplicateSignals = Set.empty[SignalReference]

    val processingUnits = expectedNodes.toList.sorted.map { nodeId =>
      val observedSignals = SignalName.values.map { signal =>
        val reference = SignalReference(signal, Some(nodeId))
        val observations = grouped.getOrElse(nodeId -> signal, Nil)
        val observedMetric = observations match {
          case Nil =>
            missingSignals += reference
            ObservedMetric(None, DataFreshness(FreshnessKind.Missing, None))
          case observation :: Nil =>
            val age = Duration.between(observation.sampledAt, observedAt)
            val metric = MetricValue.make(observation.value, observation.unit, observation.sampledAt).toOption
            if (age.isNegative || age.compareTo(maxSampleAge) > 0) {
              staleSignals += reference
              ObservedMetric(metric, DataFreshness(FreshnessKind.Stale, Some(age)))
            } else ObservedMetric(metric, DataFreshness(FreshnessKind.Fresh, Some(age)))
          case _ =>
            duplicateSignals += reference
            ObservedMetric(None, DataFreshness(FreshnessKind.Invalid, None))
        }
        signal -> observedMetric
      }.toMap
      ProcessingUnitState(nodeId, observedSignals)
    }

    val freshBySignal = SignalName.values.map { signal =>
      signal -> processingUnits.flatMap(_.signals(signal) match {
        case ObservedMetric(Some(value), DataFreshness(FreshnessKind.Fresh, _)) => Some(value)
        case _                                                                  => None
      })
    }.toMap
    val completeSignals = freshBySignal.collect {
      case (signal, values) if values.size == expectedNodes.size =>
        signal -> maximum(signal, values)
    }
    val totalRps = freshBySignal(SignalName.RequestsPerSecond) match {
      case values if values.size == expectedNodes.size => Some(total(SignalName.RequestsPerSecond, values))
      case _                                           => None
    }
    val database = for {
      pool <- completeSignals.get(SignalName.PoolAcquireP95)
      operation <- completeSignals.get(SignalName.DatabaseOperationP95)
    } yield DatabaseState(pool, operation)
    val complete = sourceStatus == MetricsSourceStatus.Available && missingSignals.isEmpty && staleSignals.isEmpty &&
      duplicateSignals.isEmpty && unknownNodes.isEmpty

    SystemState(
      observedAt,
      MetricWindow(observedAt.minusMillis(config.observationWindow.toMillis), observedAt),
      assignment.epoch,
      expectedNodes,
      processingUnits,
      database,
      ClusterState(completeSignals, totalRps),
      DataQuality(complete, sourceStatus, missingSignals, staleSignals, duplicateSignals, unknownNodes)
    )
  }

  private def maximum(signal: SignalName, values: List[MetricValue]): MetricValue = {
    val sampledAt = values.map(_.sampledAt).min
    MetricValue.make(values.map(_.value).max, signal.unit, sampledAt).toOption.get
  }

  private def total(signal: SignalName, values: List[MetricValue]): MetricValue = {
    val sampledAt = values.map(_.sampledAt).min
    MetricValue.make(values.map(_.value).sum, signal.unit, sampledAt).toOption.get
  }
}

object StateAggregator {
  val layer: URLayer[AssignmentSnapshotSource & MetricsSource & MetricsConfig, SystemStateCollector] =
    ZLayer.derive[StateAggregator]

  val module: TaskLayer[SystemStateCollector] =
    ZLayer.make[SystemStateCollector](
      AssignmentServiceSnapshotSource.module,
      PrometheusMetricsSource.module,
      MetricsConfig.module,
      layer
    )
}
