package storage.controlplane.scaling.service

import java.time.{Duration, Instant}

import storage.controlplane.config.MetricsConfig
import storage.controlplane.scaling.domain._
import storage.controlplane.systemstate.service.SystemStateCollector
import storage.controlplane.systemstate.service.StateAggregator
import zio.{Clock, Ref, TaskLayer, UIO, URLayer, ZIO, ZLayer}

final class ControllerService(
    collector: SystemStateCollector,
    scalingConfig: ScalingConfig,
    metricsConfig: MetricsConfig,
    snapshotRef: Ref[ControllerSnapshot]
) {
  def snapshot: UIO[ControllerSnapshot] = snapshotRef.get

  def run: UIO[Nothing] =
    (runOnce *> ZIO.sleep(zio.Duration.fromMillis(metricsConfig.pollInterval.toMillis))).forever

  private[service] def runOnce: UIO[Unit] =
    for {
      observedAt <- Clock.instant
      startedAt <- Clock.nanoTime
      result <- collector.collectOnce(observedAt).either
      finishedAt <- Clock.nanoTime
      cycleDuration = Duration.ofNanos(finishedAt - startedAt)
      _ <- snapshotRef.update(current => update(current, result, observedAt, cycleDuration))
    } yield ()

  private def update(
      current: ControllerSnapshot,
      result: Either[?, storage.controlplane.systemstate.domain.SystemState],
      evaluatedAt: Instant,
      cycleDuration: Duration
  ): ControllerSnapshot = result match {
    case Right(systemState) =>
      val (decision, nextPolicyState) =
        ScalingPolicy.evaluate(systemState, current.policyState, scalingConfig, evaluatedAt)
      val transition = current.decision.forall(_.kind != decision.kind)
      val transitions =
        if (transition)
          current.decisionTransitions.updated(decision.kind, current.decisionTransitions(decision.kind) + 1)
        else current.decisionTransitions
      if (systemState.dataQuality.complete)
        current.copy(
          status = ControllerStatus.Healthy,
          systemState = Some(systemState),
          decision = Some(decision),
          policyState = nextPolicyState,
          completeCycles = current.completeCycles + 1,
          lastCycleDuration = Some(cycleDuration),
          lastCompleteStateAt = Some(systemState.observedAt),
          decisionTransitions = transitions
        )
      else
        current.copy(
          status = ControllerStatus.Degraded,
          systemState = Some(systemState),
          decision = Some(decision),
          policyState = nextPolicyState,
          incompleteCycles = current.incompleteCycles + 1,
          lastCycleDuration = Some(cycleDuration),
          decisionTransitions = transitions
        )
    case Left(_) =>
      val insufficient = current.systemState.map { state =>
        ScalingDecision(
          ScalingDecisionKind.InsufficientData,
          state.expectedNodes.size,
          state.expectedNodes.size,
          evaluatedAt,
          state.assignmentEpoch,
          Nil,
          List("assignment_unavailable_or_changed")
        )
      }
      val transitions =
        if (insufficient.nonEmpty && current.decision.forall(_.kind != ScalingDecisionKind.InsufficientData))
          current.decisionTransitions.updated(
            ScalingDecisionKind.InsufficientData,
            current.decisionTransitions(ScalingDecisionKind.InsufficientData) + 1
          )
        else current.decisionTransitions
      current.copy(
        status = ControllerStatus.Degraded,
        decision = insufficient,
        policyState = current.policyState.copy(pendingDirection = None, confirmationCount = 0),
        failedCycles = current.failedCycles + 1,
        lastCycleDuration = Some(cycleDuration),
        decisionTransitions = transitions
      )
  }
}

object ControllerService {
  val layer: URLayer[SystemStateCollector & ScalingConfig & MetricsConfig, ControllerService] = ZLayer.fromZIO(
    for {
      collector <- ZIO.service[SystemStateCollector]
      scalingConfig <- ZIO.service[ScalingConfig]
      metricsConfig <- ZIO.service[MetricsConfig]
      snapshotRef <- Ref.make(ControllerSnapshot.collecting)
    } yield ControllerService(collector, scalingConfig, metricsConfig, snapshotRef)
  )

  val module: TaskLayer[ControllerService] =
    ZLayer.make[ControllerService](
      StateAggregator.module,
      ScalingConfig.module,
      MetricsConfig.module,
      layer
    )
}
