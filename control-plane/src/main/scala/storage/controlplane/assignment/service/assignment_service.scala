package storage.controlplane.assignment.service

import storage.controlplane.assignment.domain.AssignmentError.InvalidAssignment
import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot}
import storage.controlplane.assignment.repository.postgres.AssignmentPostgresRepository
import zio.{IO, Ref, TaskLayer, UIO, URLayer, ZIO, ZLayer}

final class AssignmentService(repository: AssignmentRepository, activeEpoch: Ref[Long]) {
  def current: IO[AssignmentError, AssignmentSnapshot] =
    repository.current.tap(snapshot => recordEpoch(snapshot.epoch))

  def publish(expectedEpoch: Long, snapshot: AssignmentSnapshot): IO[AssignmentError, AssignmentSnapshot] =
    for {
      _ <- ZIO
        .fail(InvalidAssignment("invalid epoch transition"))
        .when(expectedEpoch <= 0 || snapshot.epoch != expectedEpoch + 1)
      valid <- ZIO.fromEither(snapshot.validate).mapError(InvalidAssignment.apply)
      _ <- repository.publish(expectedEpoch, valid)
      _ <- recordEpoch(valid.epoch)
    } yield valid

  def epoch: UIO[Long] = activeEpoch.get

  private def recordEpoch(epoch: Long): UIO[Unit] =
    activeEpoch.update(currentEpoch => currentEpoch.max(epoch))
}

object AssignmentService {
  val layer: URLayer[AssignmentRepository, AssignmentService] = ZLayer.fromZIO(
    for {
      repository <- ZIO.service[AssignmentRepository]
      activeEpoch <- Ref.make(0L)
    } yield AssignmentService(repository, activeEpoch)
  )

  val module: TaskLayer[AssignmentService] =
    ZLayer.make[AssignmentService](AssignmentPostgresRepository.module, layer)
}
