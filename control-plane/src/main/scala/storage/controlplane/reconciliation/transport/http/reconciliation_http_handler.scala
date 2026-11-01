package storage.controlplane.reconciliation.transport.http

import storage.controlplane.reconciliation.client.advisory.AdvisoryProcessingUnitFleet
import storage.controlplane.reconciliation.service.ScalingOperationService
import storage.controlplane.reconciliation.service.ProcessingUnitFleet
import storage.controlplane.reconciliation.transport.dto.{
  ReconciliationFleetDto,
  ReconciliationOperationDto,
  ReconciliationResponse
}
import zio.{TaskLayer, UIO, URLayer, ZLayer}

final class ReconciliationHttpHandler(operations: ScalingOperationService, fleet: ProcessingUnitFleet) {
  def current: UIO[ReconciliationResponse] =
    for {
      active <- operations.active.orDie
      latest <- operations.latest.orDie
      observedFleet <- fleet.observe.option
    } yield ReconciliationResponse(
      active.map(ReconciliationOperationDto.fromDomain),
      latest.map(ReconciliationOperationDto.fromDomain),
      observedFleet.map(ReconciliationFleetDto.fromDomain)
    )
}

object ReconciliationHttpHandler {
  val layer: URLayer[ScalingOperationService & ProcessingUnitFleet, ReconciliationHttpHandler] =
    ZLayer.derive[ReconciliationHttpHandler]
  val module: TaskLayer[ReconciliationHttpHandler] =
    ZLayer.make[ReconciliationHttpHandler](
      ScalingOperationService.module,
      AdvisoryProcessingUnitFleet.module,
      layer
    )
}
