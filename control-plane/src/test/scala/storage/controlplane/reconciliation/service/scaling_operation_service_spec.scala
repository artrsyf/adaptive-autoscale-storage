package storage.controlplane.reconciliation.service

import java.time.Instant

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.reconciliation.domain.ReconciliationError.{InvalidDecision, OperationConflict}
import storage.controlplane.reconciliation.domain.{
  ReconciliationError,
  ReconciliationPhase,
  ReconciliationStatus,
  ScalingOperation
}
import storage.controlplane.scaling.domain.{ScalingDecision, ScalingDecisionKind, ScalingDirection}
import zio.test._
import zio.{IO, Ref, ZIO}

object ScalingOperationServiceSpec extends ZIOSpecDefault {
  private val now = Instant.parse("2026-10-24T12:00:00Z")
  private val assignment = AssignmentSnapshot(
    7,
    2,
    List(Node("one", "http://one:8080"), Node("two", "http://two:8080")),
    List(PartitionOwner(0, "one"), PartitionOwner(1, "two"))
  )

  override def spec = suite("ScalingOperationService")(
    test("creates one scale-out operation for the active epoch") {
      for {
        state <- Ref.make(Option.empty[ScalingOperation])
        service = ScalingOperationService(StubRepository(state))
        operation <- service.start(decision(ScalingDecisionKind.ScaleOut, 2, 3), assignment, now)
        duplicate <- service.start(decision(ScalingDecisionKind.ScaleOut, 2, 3), assignment, now).either
      } yield assertTrue(
        operation.direction == ScalingDirection.Out,
        operation.phase == ReconciliationPhase.Planning,
        operation.status == ReconciliationStatus.InProgress,
        duplicate == Left(OperationConflict)
      )
    },
    test("rejects stale, non-actionable and multi-step decisions") {
      for {
        state <- Ref.make(Option.empty[ScalingOperation])
        service = ScalingOperationService(StubRepository(state))
        stale <- service
          .start(decision(ScalingDecisionKind.ScaleOut, 2, 3).copy(assignmentEpoch = 6), assignment, now)
          .either
        maintain <- service.start(decision(ScalingDecisionKind.Maintain, 2, 2), assignment, now).either
        multiStep <- service.start(decision(ScalingDecisionKind.ScaleOut, 2, 4), assignment, now).either
      } yield assertTrue(
        stale.left.exists(_.isInstanceOf[InvalidDecision]),
        maintain.left.exists(_.isInstanceOf[InvalidDecision]),
        multiStep.left.exists(_.isInstanceOf[InvalidDecision])
      )
    },
    test("enforces direction-specific phase order and optimistic versioning") {
      for {
        state <- Ref.make(Option.empty[ScalingOperation])
        service = ScalingOperationService(StubRepository(state))
        planned <- service.start(decision(ScalingDecisionKind.ScaleIn, 2, 1), assignment, now)
        publishing <- service.advance(planned, ReconciliationPhase.PublishingAssignment, now.plusSeconds(1))
        invalid <- service.advance(publishing, ReconciliationPhase.RemovingMembers, now.plusSeconds(2)).either
        waiting <- service.advance(publishing, ReconciliationPhase.WaitingForRouters, now.plusSeconds(2))
      } yield assertTrue(
        publishing.version == 1,
        invalid.isLeft,
        waiting.version == 2,
        waiting.phase == ReconciliationPhase.WaitingForRouters
      )
    }
  )

  private def decision(kind: ScalingDecisionKind, current: Int, recommended: Int): ScalingDecision =
    ScalingDecision(kind, current, recommended, now, 7, Nil, Nil)

  private final case class StubRepository(state: Ref[Option[ScalingOperation]]) extends ScalingOperationRepository {
    override def active: IO[ReconciliationError, Option[ScalingOperation]] = state.get
    override def latest: IO[ReconciliationError, Option[ScalingOperation]] = state.get

    override def create(operation: ScalingOperation): IO[ReconciliationError, ScalingOperation] =
      state.modify {
        case None              => ZIO.succeed(operation) -> Some(operation)
        case current @ Some(_) => ZIO.fail(OperationConflict) -> current
      }.flatten

    override def save(
        expectedVersion: Long,
        operation: ScalingOperation
    ): IO[ReconciliationError, ScalingOperation] =
      state.modify {
        case Some(current) if current.id == operation.id && current.version == expectedVersion =>
          val updated = operation.copy(version = expectedVersion + 1)
          ZIO.succeed(updated) -> Some(updated)
        case current => ZIO.fail(OperationConflict) -> current
      }.flatten
  }
}
