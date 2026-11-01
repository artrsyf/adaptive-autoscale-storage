package storage.controlplane.reconciliation.domain

import java.time.Instant
import java.util.UUID

import storage.controlplane.assignment.domain.AssignmentSnapshot
import storage.controlplane.scaling.domain.ScalingDirection

enum ReconciliationPhase(val wireName: String) {
  case Planning extends ReconciliationPhase("planning")
  case Provisioning extends ReconciliationPhase("provisioning")
  case WaitingForMembers extends ReconciliationPhase("waiting_for_members")
  case PublishingAssignment extends ReconciliationPhase("publishing_assignment")
  case WaitingForRouters extends ReconciliationPhase("waiting_for_routers")
  case RemovingMembers extends ReconciliationPhase("removing_members")
  case Completed extends ReconciliationPhase("completed")
  case Failed extends ReconciliationPhase("failed")
}

object ReconciliationPhase {
  def parse(value: String): Either[String, ReconciliationPhase] =
    values.find(_.wireName == value).toRight(s"unknown reconciliation phase: $value")
}

enum ReconciliationStatus(val wireName: String) {
  case InProgress extends ReconciliationStatus("in_progress")
  case Succeeded extends ReconciliationStatus("succeeded")
  case Failed extends ReconciliationStatus("failed")
}

object ReconciliationStatus {
  def parse(value: String): Either[String, ReconciliationStatus] =
    values.find(_.wireName == value).toRight(s"unknown reconciliation status: $value")
}

final case class ScalingOperation(
    id: UUID,
    direction: ScalingDirection,
    sourceAssignmentEpoch: Long,
    sourceReplicas: ReplicaCount,
    targetReplicas: ReplicaCount,
    targetMemberIds: Set[String],
    removedMemberIds: Set[String],
    plannedAssignment: Option[AssignmentSnapshot],
    phase: ReconciliationPhase,
    status: ReconciliationStatus,
    version: Long,
    attempt: Int,
    lastErrorCode: Option[String],
    createdAt: Instant,
    updatedAt: Instant
)
