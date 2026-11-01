package storage.controlplane.reconciliation.client.advisory

import storage.controlplane.assignment.domain.Node
import storage.controlplane.reconciliation.domain.FleetError.InvalidState
import storage.controlplane.reconciliation.domain.{FleetError, FleetSnapshot, ReplicaCount}
import storage.controlplane.reconciliation.service.ProcessingUnitFleet
import zio.{IO, ULayer, ZIO, ZLayer}

final class AdvisoryProcessingUnitFleet extends ProcessingUnitFleet {
  private val disabled = InvalidState("processing unit fleet is disabled in advisory mode")

  override def observe: IO[FleetError, FleetSnapshot] = ZIO.fail(disabled)
  override def retainedMembersAfterResize(target: ReplicaCount): IO[FleetError, Set[Node]] = ZIO.fail(disabled)
  override def resize(target: ReplicaCount): IO[FleetError, Unit] = ZIO.fail(disabled)
}

object AdvisoryProcessingUnitFleet {
  val layer: ULayer[ProcessingUnitFleet] = ZLayer.succeed(AdvisoryProcessingUnitFleet())
  val module: ULayer[ProcessingUnitFleet] = layer
}
