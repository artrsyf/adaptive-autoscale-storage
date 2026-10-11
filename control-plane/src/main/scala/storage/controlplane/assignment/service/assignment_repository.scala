package storage.controlplane.assignment.service

import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot}
import zio.IO

trait AssignmentRepository {
  def current: IO[AssignmentError, AssignmentSnapshot]
  def publish(expectedEpoch: Long, snapshot: AssignmentSnapshot): IO[AssignmentError, Unit]
  def ready: IO[AssignmentError, Unit]
}
