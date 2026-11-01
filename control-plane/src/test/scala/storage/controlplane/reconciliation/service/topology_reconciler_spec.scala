package storage.controlplane.reconciliation.service

import java.time.Instant

import storage.controlplane.assignment.domain.AssignmentError.EpochConflict
import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.assignment.service.{AssignmentRepository, AssignmentService}
import storage.controlplane.reconciliation.domain.ReconciliationError.{InvalidOperation, OperationConflict}
import storage.controlplane.reconciliation.domain.{
  FleetError,
  FleetSnapshot,
  ReconciliationError,
  ReconciliationPhase,
  ReconciliationStatus,
  ReplicaCount,
  RouterTopologyError,
  RouterTopologyState,
  ScalingOperation
}
import storage.controlplane.scaling.domain.{ScalingDecision, ScalingDecisionKind}
import zio.test._
import zio.{IO, Ref, ZIO}

object TopologyReconcilerSpec extends ZIOSpecDefault {
  private val now = Instant.parse("2026-10-24T12:00:00Z")

  override def spec = suite("TopologyReconciler")(
    test("executes scale out as capacity then assignment then Router convergence") {
      for {
        fixture <- makeFixture(3)
        _ <- fixture.operations.start(decision(ScalingDecisionKind.ScaleOut, 3, 4), fixture.initial, now)
        phases <- runSteps(fixture.reconciler, 5)
        assignment <- fixture.assignmentState.get
        fleet <- fixture.fleetState.get
        events <- fixture.events.get
      } yield assertTrue(
        phases == List(
          ReconciliationPhase.Provisioning,
          ReconciliationPhase.WaitingForMembers,
          ReconciliationPhase.PublishingAssignment,
          ReconciliationPhase.WaitingForRouters,
          ReconciliationPhase.Completed
        ),
        assignment.epoch == 8,
        assignment.nodes.size == 4,
        fleet.readyMembers.size == 4,
        events == List("resize:4", "publish:8")
      )
    },
    test("executes scale in as assignment then Router convergence then capacity") {
      for {
        fixture <- makeFixture(4)
        _ <- fixture.operations.start(decision(ScalingDecisionKind.ScaleIn, 4, 3), fixture.initial, now)
        phases <- runSteps(fixture.reconciler, 4)
        assignment <- fixture.assignmentState.get
        fleet <- fixture.fleetState.get
        events <- fixture.events.get
      } yield assertTrue(
        phases == List(
          ReconciliationPhase.PublishingAssignment,
          ReconciliationPhase.WaitingForRouters,
          ReconciliationPhase.RemovingMembers,
          ReconciliationPhase.Completed
        ),
        assignment.nodes.map(_.id).toSet == Set("processing-unit-1", "processing-unit-2", "processing-unit-3"),
        fleet.readyMembers.map(_.id) == assignment.nodes.map(_.id).toSet,
        events == List("publish:8", "resize:3")
      )
    }
  )

  private final case class Fixture(
      initial: AssignmentSnapshot,
      reconciler: TopologyReconciler,
      operations: ScalingOperationService,
      assignmentState: Ref[AssignmentSnapshot],
      fleetState: Ref[FleetSnapshot],
      events: Ref[List[String]]
  )

  private def makeFixture(replicaCount: Int): ZIO[Any, Nothing, Fixture] =
    for {
      assignmentState <- Ref.make(assignment(replicaCount))
      operationState <- Ref.make(Option.empty[ScalingOperation])
      fleetState <- Ref.make(fleet(replicaCount))
      events <- Ref.make(List.empty[String])
      activeEpoch <- Ref.make(0L)
      operationService = ScalingOperationService(OperationRepositoryStub(operationState))
      assignmentService = AssignmentService(AssignmentRepositoryStub(assignmentState, events), activeEpoch)
      fleetService = ProcessingUnitFleetStub(fleetState, events)
      routerObserver = RouterTopologyObserverStub(assignmentState)
      reconciler = TopologyReconciler(
        operationService,
        assignmentService,
        fleetService,
        BalancedPartitionAllocator(),
        routerObserver
      )
    } yield Fixture(assignment(replicaCount), reconciler, operationService, assignmentState, fleetState, events)

  private def runSteps(
      reconciler: TopologyReconciler,
      count: Int
  ): IO[ReconciliationError, List[ReconciliationPhase]] =
    ZIO
      .foreach(1 to count)(step =>
        reconciler
          .runOnce(now.plusSeconds(step.toLong))
          .flatMap(ZIO.fromOption(_).orElseFail(InvalidOperation("expected active operation")))
          .map(_.phase)
      )
      .map(_.toList)

  private def decision(kind: ScalingDecisionKind, current: Int, recommended: Int): ScalingDecision =
    ScalingDecision(kind, current, recommended, now, 7, Nil, Nil)

  private def nodes(count: Int): Set[Node] =
    (1 to count).map(index => Node(s"processing-unit-$index", s"http://processing-unit-$index:8080")).toSet

  private def assignment(count: Int): AssignmentSnapshot = {
    val members = nodes(count).toList.sortBy(_.id)
    AssignmentSnapshot(
      7,
      128,
      members,
      (0 until 128).map(partition => PartitionOwner(partition, members(partition % members.size).id)).toList
    )
  }

  private def fleet(count: Int): FleetSnapshot =
    FleetSnapshot(ReplicaCount(count), ReplicaCount(count), nodes(count))

  private final case class OperationRepositoryStub(state: Ref[Option[ScalingOperation]])
      extends ScalingOperationRepository {
    override def active: IO[ReconciliationError, Option[ScalingOperation]] =
      state.get.map(_.filter(_.status == ReconciliationStatus.InProgress))

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

  private final case class AssignmentRepositoryStub(state: Ref[AssignmentSnapshot], events: Ref[List[String]])
      extends AssignmentRepository {
    override def current: IO[AssignmentError, AssignmentSnapshot] = state.get

    override def publish(expectedEpoch: Long, snapshot: AssignmentSnapshot): IO[AssignmentError, Unit] =
      state.modify { current =>
        if (current.epoch == expectedEpoch) ZIO.unit -> snapshot
        else ZIO.fail(EpochConflict) -> current
      }.flatten *> events.update(_ :+ s"publish:${snapshot.epoch}")

    override def ready: IO[AssignmentError, Unit] = ZIO.unit
  }

  private final case class ProcessingUnitFleetStub(state: Ref[FleetSnapshot], events: Ref[List[String]])
      extends ProcessingUnitFleet {
    override def observe: IO[FleetError, FleetSnapshot] = state.get

    override def retainedMembersAfterResize(target: ReplicaCount): IO[FleetError, Set[Node]] =
      state.get.map(_.readyMembers.toList.sortBy(_.id).take(target.value).toSet)

    override def resize(target: ReplicaCount): IO[FleetError, Unit] =
      state.update(_ => fleet(target.value)) *> events.update(_ :+ s"resize:${target.value}")
  }

  private final case class RouterTopologyObserverStub(assignmentState: Ref[AssignmentSnapshot])
      extends RouterTopologyObserver {
    override def current: IO[RouterTopologyError, RouterTopologyState] = assignmentState.get.map { current =>
      RouterTopologyState(current.epoch, current.partitionCount, current.nodes.map(_.id).toSet)
    }
  }
}
