package storage.controlplane.reconciliation.service

import storage.controlplane.assignment.domain.Node
import storage.controlplane.reconciliation.domain.{FleetError, FleetSnapshot, ReplicaCount}
import zio.IO

trait ProcessingUnitFleet {
  def observe: IO[FleetError, FleetSnapshot]
  def retainedMembersAfterResize(target: ReplicaCount): IO[FleetError, Set[Node]]
  def resize(target: ReplicaCount): IO[FleetError, Unit]
}
