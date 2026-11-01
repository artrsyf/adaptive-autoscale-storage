package storage.controlplane.reconciliation.service

import java.time.Instant

import storage.controlplane.assignment.domain.AssignmentSnapshot
import storage.controlplane.assignment.service.AssignmentService
import storage.controlplane.reconciliation.client.router.http.RouterHttpTopologyObserver
import storage.controlplane.reconciliation.domain.ReconciliationError.{DependencyFailure, InvalidOperation}
import storage.controlplane.reconciliation.domain.{
  FleetSnapshot,
  ReconciliationError,
  ReconciliationPhase,
  ReplicaCount,
  RouterTopologyState,
  ScalingOperation
}
import storage.controlplane.scaling.domain.ScalingDirection
import zio.{IO, URLayer, ZIO, ZLayer}

final class TopologyReconciler(
    operations: ScalingOperationService,
    assignments: AssignmentService,
    fleet: ProcessingUnitFleet,
    allocator: PartitionAllocator,
    routers: RouterTopologyObserver
) {
  def runOnce(evaluatedAt: Instant): IO[ReconciliationError, Option[ScalingOperation]] =
    operations.active.flatMap {
      case None            => ZIO.none
      case Some(operation) => reconcile(operation, evaluatedAt).asSome
    }

  private def reconcile(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] = operation.phase match {
    case ReconciliationPhase.Planning => plan(operation, evaluatedAt)
    case ReconciliationPhase.Provisioning =>
      resize(operation.targetReplicas) *>
        operations.advance(operation, ReconciliationPhase.WaitingForMembers, evaluatedAt)
    case ReconciliationPhase.WaitingForMembers                      => waitForMembers(operation, evaluatedAt)
    case ReconciliationPhase.PublishingAssignment                   => publishAssignment(operation, evaluatedAt)
    case ReconciliationPhase.WaitingForRouters                      => waitForRouters(operation, evaluatedAt)
    case ReconciliationPhase.RemovingMembers                        => removeMembers(operation, evaluatedAt)
    case ReconciliationPhase.Completed | ReconciliationPhase.Failed => ZIO.succeed(operation)
  }

  private def plan(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] = operation.direction match {
    case ScalingDirection.Out =>
      operations.advance(operation, ReconciliationPhase.Provisioning, evaluatedAt)
    case ScalingDirection.In =>
      for {
        current <- currentAssignment(operation)
        targetMembers <- fleet
          .retainedMembersAfterResize(operation.targetReplicas)
          .mapError(DependencyFailure("processing_unit_fleet", _))
        targetIds = targetMembers.map(_.id)
        currentIds = current.nodes.map(_.id).toSet
        _ <- ZIO
          .fail(InvalidOperation("scale-in target members must be a subset of active assignment"))
          .unless(targetIds.subsetOf(currentIds) && targetIds.size == operation.targetReplicas.value)
        planned <- ZIO
          .fromEither(allocator.allocate(current, targetMembers))
          .mapError(error => InvalidOperation(error.getMessage))
        updated <- operations.recordPlan(operation, targetIds, currentIds -- targetIds, planned, evaluatedAt)
      } yield updated
  }

  private def waitForMembers(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      observed <- observeFleet
      current <- currentAssignment(operation)
      ready = observed.readyMembers
      updated <-
        if (fleetMatchesTarget(observed, operation.targetReplicas))
          ZIO
            .fromEither(allocator.allocate(current, ready))
            .mapError(error => InvalidOperation(error.getMessage))
            .flatMap(planned =>
              operations.recordPlan(
                operation,
                ready.map(_.id),
                Set.empty,
                planned,
                evaluatedAt
              )
            )
        else ZIO.succeed(operation)
    } yield updated

  private def publishAssignment(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      planned <- ZIO
        .fromOption(operation.plannedAssignment)
        .orElseFail(InvalidOperation("publishing phase requires a planned assignment"))
      current <- assignments.current.mapError(DependencyFailure("assignment", _))
      _ <-
        if (current.epoch == operation.sourceAssignmentEpoch)
          assignments
            .publish(operation.sourceAssignmentEpoch, planned)
            .mapError(DependencyFailure("assignment", _))
            .unit
        else if (current == planned) ZIO.unit
        else ZIO.fail(InvalidOperation("active assignment conflicts with persisted scaling plan"))
      updated <- operations.advance(operation, ReconciliationPhase.WaitingForRouters, evaluatedAt)
    } yield updated

  private def waitForRouters(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      planned <- ZIO
        .fromOption(operation.plannedAssignment)
        .orElseFail(InvalidOperation("Router convergence requires a planned assignment"))
      topology <- routers.current.mapError(DependencyFailure("router_topology", _))
      nextPhase = operation.direction match {
        case ScalingDirection.Out => ReconciliationPhase.Completed
        case ScalingDirection.In  => ReconciliationPhase.RemovingMembers
      }
      updated <-
        if (routerMatches(topology, planned)) operations.advance(operation, nextPhase, evaluatedAt)
        else ZIO.succeed(operation)
    } yield updated

  private def removeMembers(
      operation: ScalingOperation,
      evaluatedAt: Instant
  ): IO[ReconciliationError, ScalingOperation] =
    for {
      _ <- resize(operation.targetReplicas)
      observed <- observeFleet
      updated <-
        if (
          fleetMatchesTarget(observed, operation.targetReplicas) &&
          observed.readyMembers.map(_.id) == operation.targetMemberIds
        ) operations.advance(operation, ReconciliationPhase.Completed, evaluatedAt)
        else ZIO.succeed(operation)
    } yield updated

  private def currentAssignment(operation: ScalingOperation): IO[ReconciliationError, AssignmentSnapshot] =
    assignments.current.mapError(DependencyFailure("assignment", _)).flatMap { current =>
      if (current.epoch == operation.sourceAssignmentEpoch) ZIO.succeed(current)
      else ZIO.fail(InvalidOperation("active assignment changed before plan publication"))
    }

  private def resize(target: ReplicaCount): IO[ReconciliationError, Unit] =
    fleet.resize(target).mapError(DependencyFailure("processing_unit_fleet", _))

  private def observeFleet: IO[ReconciliationError, FleetSnapshot] =
    fleet.observe.mapError(DependencyFailure("processing_unit_fleet", _))

  private def fleetMatchesTarget(observed: FleetSnapshot, target: ReplicaCount): Boolean =
    observed.desiredReplicas == target && observed.observedReplicas == target &&
      observed.readyMembers.size == target.value

  private def routerMatches(topology: RouterTopologyState, assignment: AssignmentSnapshot): Boolean =
    topology.epoch == assignment.epoch && topology.partitionCount == assignment.partitionCount &&
      topology.nodeIds == assignment.nodes.map(_.id).toSet
}

object TopologyReconciler {
  val layer: URLayer[
    ScalingOperationService & AssignmentService & ProcessingUnitFleet & PartitionAllocator & RouterTopologyObserver,
    TopologyReconciler
  ] = ZLayer.derive[TopologyReconciler]

  val module: ZLayer[ProcessingUnitFleet, Throwable, TopologyReconciler] = ZLayer.makeSome[
    ProcessingUnitFleet,
    TopologyReconciler
  ](
    ScalingOperationService.module,
    AssignmentService.module,
    BalancedPartitionAllocator.module,
    RouterHttpTopologyObserver.module,
    layer
  )
}
