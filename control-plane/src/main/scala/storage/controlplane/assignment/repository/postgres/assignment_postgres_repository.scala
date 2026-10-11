package storage.controlplane.assignment.repository.postgres

import doobie._
import doobie.implicits._
import storage.controlplane.assignment.domain.AssignmentError.{EpochConflict, InvalidAssignment, Unavailable}
import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.assignment.service.AssignmentRepository
import zio.interop.catz._
import zio.{IO, Task, TaskLayer, URLayer, ZLayer}

final class AssignmentPostgresRepository(transactor: Transactor[Task]) extends AssignmentRepository {
  override def current: IO[AssignmentError, AssignmentSnapshot] =
    mapDatabaseErrors(
      (for {
        _ <- sql"SET TRANSACTION ISOLATION LEVEL REPEATABLE READ READ ONLY".update.run
        state <- sql"SELECT epoch, partition_count FROM assignment_state WHERE singleton_id = true"
          .query[(Long, Int)]
          .unique
        nodes <- sql"SELECT node_id, url FROM assignment_nodes WHERE epoch = ${state._1} ORDER BY node_id"
          .query[(String, String)]
          .to[List]
        owners <-
          sql"SELECT partition_id, owner_node_id FROM partition_assignments WHERE epoch = ${state._1} ORDER BY partition_id"
            .query[(Int, String)]
            .to[List]
        snapshot = AssignmentSnapshot(
          state._1,
          state._2,
          nodes.map { case (id, url) => Node(id, url) },
          owners.map { case (partition, nodeId) => PartitionOwner(partition, nodeId) }
        )
        valid <- snapshot.validate.fold(
          details => FC.raiseError[AssignmentSnapshot](InvalidAssignment(details)),
          FC.pure
        )
      } yield valid).transact(transactor)
    )

  override def publish(expectedEpoch: Long, snapshot: AssignmentSnapshot): IO[AssignmentError, Unit] =
    mapDatabaseErrors(
      (for {
        state <- sql"SELECT epoch, partition_count FROM assignment_state WHERE singleton_id = true FOR UPDATE"
          .query[(Long, Int)]
          .unique
        _ <- if (state._1 != expectedEpoch) FC.raiseError[Unit](EpochConflict) else FC.unit
        _ <-
          if (snapshot.epoch != state._1 + 1 || snapshot.partitionCount != state._2)
            FC.raiseError[Unit](InvalidAssignment("invalid assignment transition"))
          else FC.unit
        _ <- Update[(Long, String, String)](
          "INSERT INTO assignment_nodes(epoch,node_id,url) VALUES(?,?,?)"
        ).updateMany(snapshot.nodes.map(node => (snapshot.epoch, node.id, node.url)))
        _ <- Update[(Long, Int, String)](
          "INSERT INTO partition_assignments(epoch,partition_id,owner_node_id) VALUES(?,?,?)"
        ).updateMany(snapshot.assignments.map(owner => (snapshot.epoch, owner.partition, owner.nodeId)))
        updated <-
          sql"UPDATE assignment_state SET epoch=${snapshot.epoch},updated_at=clock_timestamp() WHERE singleton_id=true AND epoch=$expectedEpoch".update.run
        _ <- if (updated == 1) FC.unit else FC.raiseError[Unit](EpochConflict)
      } yield ()).transact(transactor)
    )

  override def ready: IO[AssignmentError, Unit] =
    mapDatabaseErrors(
      sql"""SELECT to_regclass('public.assignment_state') IS NOT NULL
                    AND to_regclass('public.assignment_nodes') IS NOT NULL
                    AND to_regclass('public.partition_assignments') IS NOT NULL"""
        .query[Boolean]
        .unique
        .flatMap(ready =>
          if (ready) FC.unit else FC.raiseError[Unit](new IllegalStateException("assignment schema missing"))
        )
        .transact(transactor)
    )

  private def mapDatabaseErrors[A](effect: Task[A]): IO[AssignmentError, A] =
    effect.mapError {
      case assignmentError: AssignmentError => assignmentError
      case error                            => Unavailable(error)
    }
}

object AssignmentPostgresRepository {
  val layer: URLayer[Transactor[Task], AssignmentRepository] = ZLayer.derive[AssignmentPostgresRepository]
  val module: TaskLayer[AssignmentRepository] =
    ZLayer.make[AssignmentRepository](PostgresDataSource.module, layer)
}
