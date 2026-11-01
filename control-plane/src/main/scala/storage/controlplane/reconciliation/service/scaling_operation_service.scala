package storage.controlplane.reconciliation.service

import java.time.Instant
import java.util.UUID

import storage.controlplane.assignment.domain.AssignmentSnapshot
import storage.controlplane.reconciliation.domain.ReconciliationError.{InvalidDecision, InvalidOperation}
import storage.controlplane.reconciliation.domain.{
  ReconciliationError,
  ReconciliationPhase,
  ReconciliationStatus,
  ReplicaCount,
  ScalingOperation
}
import storage.controlplane.reconciliation.repository.postgres.ScalingOperationPostgresRepository
import storage.controlplane.scaling.domain.{ScalingDecision, ScalingDecisionKind, ScalingDirection}
import zio.{IO, TaskLayer, URLayer, ZIO, ZLayer}

final class ScalingOperationService(repository: ScalingOperationRepository) {
  def active: IO[ReconciliationError, Option[ScalingOperation]] = repository.active
  def latest: IO[ReconciliationError, Option[ScalingOperation]] = repository.latest

  def start(
      decision: ScalingDecision,
      assignment: AssignmentSnapshot,
      startedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      direction <- directionFrom(decision.kind)
      _ <- ZIO
        .fail(InvalidDecision("decision epoch or replica count does not match active assignment"))
        .unless(
          decision.assignmentEpoch == assignment.epoch &&
            decision.currentReplicas == assignment.nodes.size
        )
      _ <- ZIO
        .fail(InvalidDecision("actionable decision must change replicas by one"))
        .unless(math.abs(decision.recommendedReplicas - decision.currentReplicas) == 1)
      _ <- ZIO
        .fail(InvalidDecision("decision direction does not match recommended replicas"))
        .unless(
          direction == ScalingDirection.Out && decision.recommendedReplicas > decision.currentReplicas ||
            direction == ScalingDirection.In && decision.recommendedReplicas < decision.currentReplicas
        )
      operation = ScalingOperation(
        UUID.randomUUID(),
        direction,
        assignment.epoch,
        ReplicaCount(decision.currentReplicas),
        ReplicaCount(decision.recommendedReplicas),
        Set.empty,
        Set.empty,
        None,
        ReconciliationPhase.Planning,
        ReconciliationStatus.InProgress,
        0,
        0,
        None,
        startedAt,
        startedAt
      )
      created <- repository.create(operation)
    } yield created

  def advance(
      operation: ScalingOperation,
      nextPhase: ReconciliationPhase,
      updatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      _ <- ZIO
        .fail(InvalidOperation("only an in-progress operation can advance"))
        .unless(operation.status == ReconciliationStatus.InProgress)
      _ <- ZIO
        .fail(InvalidOperation(s"invalid ${operation.direction} transition: ${operation.phase} -> $nextPhase"))
        .unless(allowedNext(operation.direction, operation.phase).contains(nextPhase))
      nextStatus = nextPhase match {
        case ReconciliationPhase.Completed => ReconciliationStatus.Succeeded
        case ReconciliationPhase.Failed    => ReconciliationStatus.Failed
        case _                             => ReconciliationStatus.InProgress
      }
      updated <- repository.save(
        operation.version,
        operation.copy(phase = nextPhase, status = nextStatus, lastErrorCode = None, updatedAt = updatedAt)
      )
    } yield updated

  def recordPlan(
      operation: ScalingOperation,
      targetMemberIds: Set[String],
      removedMemberIds: Set[String],
      assignment: AssignmentSnapshot,
      updatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      _ <- ZIO
        .fail(InvalidOperation("planned assignment must be the next source epoch"))
        .unless(assignment.epoch == operation.sourceAssignmentEpoch + 1)
      _ <- ZIO
        .fail(InvalidOperation("planned assignment members must match target members"))
        .unless(assignment.nodes.map(_.id).toSet == targetMemberIds)
      _ <- ZIO
        .fail(InvalidOperation("planned target member count must match target replicas"))
        .unless(targetMemberIds.size == operation.targetReplicas.value)
      updated <- repository.save(
        operation.version,
        operation.copy(
          targetMemberIds = targetMemberIds,
          removedMemberIds = removedMemberIds,
          plannedAssignment = Some(assignment),
          phase = ReconciliationPhase.PublishingAssignment,
          lastErrorCode = None,
          updatedAt = updatedAt
        )
      )
    } yield updated

  def recordFailure(
      operation: ScalingOperation,
      errorCode: String,
      updatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    if (errorCode.trim.isEmpty || errorCode.length > 128)
      ZIO.fail(InvalidOperation("failure code must contain between 1 and 128 characters"))
    else
      repository.save(
        operation.version,
        operation.copy(
          attempt = operation.attempt + 1,
          lastErrorCode = Some(errorCode),
          updatedAt = updatedAt
        )
      )

  private def directionFrom(kind: ScalingDecisionKind): IO[ReconciliationError, ScalingDirection] = kind match {
    case ScalingDecisionKind.ScaleOut => ZIO.succeed(ScalingDirection.Out)
    case ScalingDecisionKind.ScaleIn  => ZIO.succeed(ScalingDirection.In)
    case _                            => ZIO.fail(InvalidDecision("decision is not actionable"))
  }

  private def allowedNext(
      direction: ScalingDirection,
      phase: ReconciliationPhase
  ): Option[ReconciliationPhase] = (direction, phase) match {
    case (ScalingDirection.Out, ReconciliationPhase.Planning)          => Some(ReconciliationPhase.Provisioning)
    case (ScalingDirection.Out, ReconciliationPhase.Provisioning)      => Some(ReconciliationPhase.WaitingForMembers)
    case (ScalingDirection.Out, ReconciliationPhase.WaitingForMembers) => Some(ReconciliationPhase.PublishingAssignment)
    case (ScalingDirection.Out, ReconciliationPhase.PublishingAssignment) => Some(ReconciliationPhase.WaitingForRouters)
    case (ScalingDirection.Out, ReconciliationPhase.WaitingForRouters)    => Some(ReconciliationPhase.Completed)
    case (ScalingDirection.In, ReconciliationPhase.Planning) => Some(ReconciliationPhase.PublishingAssignment)
    case (ScalingDirection.In, ReconciliationPhase.PublishingAssignment) => Some(ReconciliationPhase.WaitingForRouters)
    case (ScalingDirection.In, ReconciliationPhase.WaitingForRouters)    => Some(ReconciliationPhase.RemovingMembers)
    case (ScalingDirection.In, ReconciliationPhase.RemovingMembers)      => Some(ReconciliationPhase.Completed)
    case (_, ReconciliationPhase.Failed)                                 => None
    case (_, ReconciliationPhase.Completed)                              => None
    case _                                                               => Some(ReconciliationPhase.Failed)
  }
}

object ScalingOperationService {
  val layer: URLayer[ScalingOperationRepository, ScalingOperationService] = ZLayer.derive[ScalingOperationService]
  val module: TaskLayer[ScalingOperationService] =
    ZLayer.make[ScalingOperationService](ScalingOperationPostgresRepository.module, layer)
}
