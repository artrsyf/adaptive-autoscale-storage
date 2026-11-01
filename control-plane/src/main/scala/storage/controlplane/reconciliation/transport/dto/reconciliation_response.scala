package storage.controlplane.reconciliation.transport.dto

import storage.controlplane.reconciliation.domain.{FleetSnapshot, ScalingOperation}
import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonMemberNames, JsonCodec, SnakeCase}

@jsonMemberNames(SnakeCase)
final case class ReconciliationOperationDto(
    operationId: String,
    direction: String,
    sourceAssignmentEpoch: Long,
    sourceReplicas: Int,
    targetReplicas: Int,
    phase: String,
    status: String,
    attempt: Int,
    lastErrorCode: Option[String],
    targetMemberIds: List[String],
    removedMemberIds: List[String]
) derives JsonCodec,
      Loggable,
      Schema,
      CanEqual

object ReconciliationOperationDto {
  def fromDomain(operation: ScalingOperation): ReconciliationOperationDto = ReconciliationOperationDto(
    operation.id.toString,
    operation.direction.toString.toLowerCase,
    operation.sourceAssignmentEpoch,
    operation.sourceReplicas.value,
    operation.targetReplicas.value,
    operation.phase.wireName,
    operation.status.wireName,
    operation.attempt,
    operation.lastErrorCode,
    operation.targetMemberIds.toList.sorted,
    operation.removedMemberIds.toList.sorted
  )
}

@jsonMemberNames(SnakeCase)
final case class ReconciliationFleetDto(
    desiredReplicas: Int,
    observedReplicas: Int,
    readyMemberIds: List[String]
) derives JsonCodec,
      Loggable,
      Schema,
      CanEqual

object ReconciliationFleetDto {
  def fromDomain(fleet: FleetSnapshot): ReconciliationFleetDto = ReconciliationFleetDto(
    fleet.desiredReplicas.value,
    fleet.observedReplicas.value,
    fleet.readyMembers.map(_.id).toList.sorted
  )
}

@jsonMemberNames(SnakeCase)
final case class ReconciliationResponse(
    active: Option[ReconciliationOperationDto],
    latest: Option[ReconciliationOperationDto],
    fleet: Option[ReconciliationFleetDto]
) derives JsonCodec,
      Loggable,
      Schema,
      CanEqual
