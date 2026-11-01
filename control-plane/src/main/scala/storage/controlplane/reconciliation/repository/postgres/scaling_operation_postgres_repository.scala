package storage.controlplane.reconciliation.repository.postgres

import java.time.Instant
import java.util.UUID
import java.sql.SQLException

import doobie._
import doobie.implicits._
import doobie.postgres.implicits._
import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.assignment.repository.postgres.PostgresDataSource
import storage.controlplane.reconciliation.domain.ReconciliationError.{InvalidOperation, OperationConflict, Unavailable}
import storage.controlplane.reconciliation.domain.{
  ReconciliationError,
  ReconciliationPhase,
  ReconciliationStatus,
  ReplicaCount,
  ScalingOperation
}
import storage.controlplane.reconciliation.service.ScalingOperationRepository
import storage.controlplane.scaling.domain.ScalingDirection
import zio.interop.catz._
import zio.{IO, Task, TaskLayer, URLayer, ZLayer}

final class ScalingOperationPostgresRepository(transactor: Transactor[Task]) extends ScalingOperationRepository {
  override def active: IO[ReconciliationError, Option[ScalingOperation]] =
    mapDatabaseErrors(activeOperation.transact(transactor))

  override def latest: IO[ReconciliationError, Option[ScalingOperation]] =
    mapDatabaseErrors(latestOperation.transact(transactor))

  override def create(operation: ScalingOperation): IO[ReconciliationError, ScalingOperation] =
    mapDatabaseErrors(
      (for {
        _ <- validateForCreate(operation)
        _ <- insertOperation(operation)
        _ <- replaceDetails(operation)
      } yield operation).transact(transactor)
    )

  override def save(
      expectedVersion: Long,
      operation: ScalingOperation
  ): IO[ReconciliationError, ScalingOperation] = {
    val updated = operation.copy(version = expectedVersion + 1)
    mapDatabaseErrors(
      (for {
        affected <- updateOperation(expectedVersion, updated)
        _ <- if (affected == 1) FC.unit else FC.raiseError[Unit](OperationConflict)
        _ <- replaceDetails(updated)
      } yield updated).transact(transactor)
    )
  }

  private val activeOperation: ConnectionIO[Option[ScalingOperation]] =
    sql"""SELECT operation_id, direction, source_assignment_epoch, source_replicas, target_replicas,
                  phase, status, version, attempt, last_error_code,
                  planned_assignment_epoch, planned_partition_count, created_at, updated_at
           FROM scaling_operations
           WHERE status = 'in_progress'
           ORDER BY created_at DESC
           LIMIT 1"""
      .query[OperationRow]
      .option
      .flatMap {
        case None      => FC.pure(None)
        case Some(row) => readOperation(row).map(Some(_))
      }

  private val latestOperation: ConnectionIO[Option[ScalingOperation]] =
    sql"""SELECT operation_id, direction, source_assignment_epoch, source_replicas, target_replicas,
                  phase, status, version, attempt, last_error_code,
                  planned_assignment_epoch, planned_partition_count, created_at, updated_at
           FROM scaling_operations
           ORDER BY created_at DESC
           LIMIT 1"""
      .query[OperationRow]
      .option
      .flatMap {
        case None      => FC.pure(None)
        case Some(row) => readOperation(row).map(Some(_))
      }

  private def readOperation(row: OperationRow): ConnectionIO[ScalingOperation] =
    for {
      direction <- parseDirection(row.direction)
      phase <- fromEither(ReconciliationPhase.parse(row.phase))
      status <- fromEither(ReconciliationStatus.parse(row.status))
      members <- sql"SELECT node_id, role FROM scaling_operation_members WHERE operation_id = ${row.id}"
        .query[(String, String)]
        .to[List]
      plannedAssignment <- readPlannedAssignment(row)
    } yield ScalingOperation(
      row.id,
      direction,
      row.sourceAssignmentEpoch,
      ReplicaCount(row.sourceReplicas),
      ReplicaCount(row.targetReplicas),
      members.collect { case (nodeId, "target") => nodeId }.toSet,
      members.collect { case (nodeId, "removed") => nodeId }.toSet,
      plannedAssignment,
      phase,
      status,
      row.version,
      row.attempt,
      row.lastErrorCode,
      row.createdAt,
      row.updatedAt
    )

  private def readPlannedAssignment(row: OperationRow): ConnectionIO[Option[AssignmentSnapshot]] =
    (row.plannedAssignmentEpoch, row.plannedPartitionCount) match {
      case (None, None) => FC.pure(None)
      case (Some(epoch), Some(partitionCount)) =>
        for {
          nodes <- sql"""SELECT node_id, url FROM scaling_operation_assignment_nodes
                         WHERE operation_id = ${row.id} ORDER BY node_id"""
            .query[(String, String)]
            .to[List]
          assignments <- sql"""SELECT partition_id, owner_node_id FROM scaling_operation_partition_assignments
                               WHERE operation_id = ${row.id} ORDER BY partition_id"""
            .query[(Int, String)]
            .to[List]
          snapshot = AssignmentSnapshot(
            epoch,
            partitionCount,
            nodes.map { case (nodeId, url) => Node(nodeId, url) },
            assignments.map { case (partition, nodeId) => PartitionOwner(partition, nodeId) }
          )
          valid <- fromEither(snapshot.validate)
        } yield Some(valid)
      case _ => FC.raiseError(InvalidOperation("incomplete planned assignment metadata"))
    }

  private def validateForCreate(operation: ScalingOperation): ConnectionIO[Unit] =
    if (operation.version != 0 || operation.status != ReconciliationStatus.InProgress)
      FC.raiseError(InvalidOperation("new scaling operation must be in progress at version zero"))
    else FC.unit

  private def insertOperation(operation: ScalingOperation): ConnectionIO[Unit] = {
    val plannedEpoch = operation.plannedAssignment.map(_.epoch)
    val plannedPartitionCount = operation.plannedAssignment.map(_.partitionCount)
    sql"""INSERT INTO scaling_operations(
            operation_id, direction, source_assignment_epoch, source_replicas, target_replicas,
            phase, status, version, attempt, last_error_code,
            planned_assignment_epoch, planned_partition_count, created_at, updated_at
          ) VALUES (
            ${operation.id}, ${directionName(operation.direction)}, ${operation.sourceAssignmentEpoch},
            ${operation.sourceReplicas.value}, ${operation.targetReplicas.value}, ${operation.phase.wireName},
            ${operation.status.wireName}, ${operation.version}, ${operation.attempt}, ${operation.lastErrorCode},
            $plannedEpoch, $plannedPartitionCount, ${operation.createdAt}, ${operation.updatedAt}
          )""".update.run.map(_ => ())
  }

  private def updateOperation(expectedVersion: Long, operation: ScalingOperation): ConnectionIO[Int] = {
    val plannedEpoch = operation.plannedAssignment.map(_.epoch)
    val plannedPartitionCount = operation.plannedAssignment.map(_.partitionCount)
    sql"""UPDATE scaling_operations SET
            direction = ${directionName(operation.direction)},
            source_assignment_epoch = ${operation.sourceAssignmentEpoch},
            source_replicas = ${operation.sourceReplicas.value},
            target_replicas = ${operation.targetReplicas.value},
            phase = ${operation.phase.wireName},
            status = ${operation.status.wireName},
            version = ${operation.version},
            attempt = ${operation.attempt},
            last_error_code = ${operation.lastErrorCode},
            planned_assignment_epoch = $plannedEpoch,
            planned_partition_count = $plannedPartitionCount,
            updated_at = ${operation.updatedAt}
          WHERE operation_id = ${operation.id} AND version = $expectedVersion""".update.run
  }

  private def replaceDetails(operation: ScalingOperation): ConnectionIO[Unit] =
    for {
      _ <- sql"DELETE FROM scaling_operation_partition_assignments WHERE operation_id = ${operation.id}".update.run
      _ <- sql"DELETE FROM scaling_operation_assignment_nodes WHERE operation_id = ${operation.id}".update.run
      _ <- sql"DELETE FROM scaling_operation_members WHERE operation_id = ${operation.id}".update.run
      _ <- Update[(UUID, String, String)](
        "INSERT INTO scaling_operation_members(operation_id,node_id,role) VALUES(?,?,?)"
      ).updateMany(
        operation.targetMemberIds.toList.sorted.map(nodeId => (operation.id, nodeId, "target")) ++
          operation.removedMemberIds.toList.sorted.map(nodeId => (operation.id, nodeId, "removed"))
      )
      _ <- operation.plannedAssignment.fold(FC.unit)(assignment => insertPlannedAssignment(operation.id, assignment))
    } yield ()

  private def insertPlannedAssignment(operationId: UUID, assignment: AssignmentSnapshot): ConnectionIO[Unit] =
    for {
      valid <- fromEither(assignment.validate)
      _ <- Update[(UUID, String, String)](
        "INSERT INTO scaling_operation_assignment_nodes(operation_id,node_id,url) VALUES(?,?,?)"
      ).updateMany(valid.nodes.map(node => (operationId, node.id, node.url)))
      _ <- Update[(UUID, Int, String)](
        "INSERT INTO scaling_operation_partition_assignments(operation_id,partition_id,owner_node_id) VALUES(?,?,?)"
      ).updateMany(valid.assignments.map(owner => (operationId, owner.partition, owner.nodeId)))
    } yield ()

  private def parseDirection(value: String): ConnectionIO[ScalingDirection] = value match {
    case "out" => FC.pure(ScalingDirection.Out)
    case "in"  => FC.pure(ScalingDirection.In)
    case _     => FC.raiseError(InvalidOperation(s"unknown scaling direction: $value"))
  }

  private def directionName(direction: ScalingDirection): String = direction match {
    case ScalingDirection.Out => "out"
    case ScalingDirection.In  => "in"
  }

  private def fromEither[A](result: Either[String, A]): ConnectionIO[A] =
    result.fold(details => FC.raiseError(InvalidOperation(details)), FC.pure)

  private def mapDatabaseErrors[A](effect: Task[A]): IO[ReconciliationError, A] =
    effect.mapError {
      case reconciliationError: ReconciliationError                  => reconciliationError
      case sqlError: SQLException if sqlError.getSQLState == "23505" => OperationConflict
      case error                                                     => Unavailable(error)
    }

  private final case class OperationRow(
      id: UUID,
      direction: String,
      sourceAssignmentEpoch: Long,
      sourceReplicas: Int,
      targetReplicas: Int,
      phase: String,
      status: String,
      version: Long,
      attempt: Int,
      lastErrorCode: Option[String],
      plannedAssignmentEpoch: Option[Long],
      plannedPartitionCount: Option[Int],
      createdAt: Instant,
      updatedAt: Instant
  )
}

object ScalingOperationPostgresRepository {
  val layer: URLayer[Transactor[Task], ScalingOperationRepository] =
    ZLayer.derive[ScalingOperationPostgresRepository]
  val module: TaskLayer[ScalingOperationRepository] =
    ZLayer.make[ScalingOperationRepository](PostgresDataSource.module, layer)
}
