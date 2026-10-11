package storage.controlplane.scaling.domain

import java.time.{Duration, Instant}

import storage.controlplane.systemstate.domain.SystemState

enum ControllerStatus(val wireName: String) {
  case Collecting extends ControllerStatus("collecting")
  case Healthy extends ControllerStatus("healthy")
  case Degraded extends ControllerStatus("degraded")
}

final case class ControllerSnapshot(
    status: ControllerStatus,
    systemState: Option[SystemState],
    decision: Option[ScalingDecision],
    policyState: PolicyState,
    completeCycles: Long,
    incompleteCycles: Long,
    failedCycles: Long,
    lastCycleDuration: Option[Duration],
    lastCompleteStateAt: Option[Instant],
    decisionTransitions: Map[ScalingDecisionKind, Long]
)

object ControllerSnapshot {
  val collecting: ControllerSnapshot = ControllerSnapshot(
    ControllerStatus.Collecting,
    None,
    None,
    PolicyState.empty,
    0,
    0,
    0,
    None,
    None,
    ScalingDecisionKind.values.map(_ -> 0L).toMap
  )
}
