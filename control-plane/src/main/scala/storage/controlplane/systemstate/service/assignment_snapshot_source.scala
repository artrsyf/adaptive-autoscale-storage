package storage.controlplane.systemstate.service

import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot}
import storage.controlplane.assignment.service.AssignmentService
import zio.{IO, TaskLayer, URLayer, ZLayer}

trait AssignmentSnapshotSource {
  def current: IO[AssignmentError, AssignmentSnapshot]
}

final class AssignmentServiceSnapshotSource(service: AssignmentService) extends AssignmentSnapshotSource {
  override def current: IO[AssignmentError, AssignmentSnapshot] = service.current
}

object AssignmentServiceSnapshotSource {
  val layer: URLayer[AssignmentService, AssignmentSnapshotSource] = ZLayer.derive[AssignmentServiceSnapshotSource]
  val module: TaskLayer[AssignmentSnapshotSource] =
    ZLayer.make[AssignmentSnapshotSource](AssignmentService.module, layer)
}
