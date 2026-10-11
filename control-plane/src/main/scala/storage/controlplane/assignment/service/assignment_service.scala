package storage.controlplane.assignment.service

import storage.controlplane.assignment.domain.AssignmentError.InvalidAssignment
import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot}
import zio.{IO, Ref, UIO, ZIO}

final class AssignmentService private (repository: AssignmentRepository, activeEpoch: Ref[Long]) {
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
  def make(repository: AssignmentRepository): UIO[AssignmentService] =
    Ref.make(0L).map(new AssignmentService(repository, _))
}
