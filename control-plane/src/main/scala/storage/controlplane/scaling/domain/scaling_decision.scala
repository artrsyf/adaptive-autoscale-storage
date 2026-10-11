package storage.controlplane.scaling.domain

import java.time.Instant

import storage.controlplane.systemstate.domain.{MetricUnit, SignalName}

enum ScalingDecisionKind(val wireName: String) {
  case ScaleOut extends ScalingDecisionKind("scale_out")
  case ScaleIn extends ScalingDecisionKind("scale_in")
  case Maintain extends ScalingDecisionKind("maintain")
  case InsufficientData extends ScalingDecisionKind("insufficient_data")
}

enum ScalingDirection {
  case Out
  case In
}

final case class ScalingReason(
    signal: SignalName,
    value: Double,
    unit: MetricUnit,
    comparison: String,
    threshold: Double
)

final case class ScalingDecision(
    kind: ScalingDecisionKind,
    currentReplicas: Int,
    recommendedReplicas: Int,
    evaluatedAt: Instant,
    assignmentEpoch: Long,
    reasons: List[ScalingReason],
    blockedBy: List[String]
)

final case class PolicyState(
    pendingDirection: Option[ScalingDirection],
    confirmationCount: Int,
    lastRecommendationAt: Option[Instant]
)

object PolicyState {
  val empty: PolicyState = PolicyState(None, 0, None)
}
