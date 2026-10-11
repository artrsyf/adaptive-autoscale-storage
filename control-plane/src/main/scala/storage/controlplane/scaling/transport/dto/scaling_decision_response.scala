package storage.controlplane.scaling.transport.dto

import storage.controlplane.scaling.domain.{ControllerSnapshot, ScalingDecision, ScalingReason}
import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonMemberNames, JsonCodec, SnakeCase}

@jsonMemberNames(SnakeCase)
final case class ScalingReasonDto(
    signal: String,
    value: Double,
    unit: String,
    comparison: String,
    threshold: Double
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object ScalingReasonDto {
  def fromDomain(reason: ScalingReason): ScalingReasonDto =
    ScalingReasonDto(reason.signal.wireName, reason.value, reason.unit.wireName, reason.comparison, reason.threshold)
}

@jsonMemberNames(SnakeCase)
final case class ScalingDecisionDto(
    kind: String,
    currentReplicas: Int,
    recommendedReplicas: Int,
    evaluatedAt: String,
    assignmentEpoch: Long,
    reasons: List[ScalingReasonDto],
    blockedBy: List[String]
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object ScalingDecisionDto {
  def fromDomain(decision: ScalingDecision): ScalingDecisionDto = ScalingDecisionDto(
    decision.kind.wireName,
    decision.currentReplicas,
    decision.recommendedReplicas,
    decision.evaluatedAt.toString,
    decision.assignmentEpoch,
    decision.reasons.map(ScalingReasonDto.fromDomain),
    decision.blockedBy
  )
}

@jsonMemberNames(SnakeCase)
final case class ControlDecisionResponse(
    status: String,
    decision: Option[ScalingDecisionDto],
    confirmationCount: Int,
    pendingDirection: Option[String]
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object ControlDecisionResponse {
  def fromDomain(snapshot: ControllerSnapshot): ControlDecisionResponse = ControlDecisionResponse(
    snapshot.status.wireName,
    snapshot.decision.map(ScalingDecisionDto.fromDomain),
    snapshot.policyState.confirmationCount,
    snapshot.policyState.pendingDirection.map(_.toString.toLowerCase)
  )
}
