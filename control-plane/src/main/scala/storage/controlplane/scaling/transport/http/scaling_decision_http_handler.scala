package storage.controlplane.scaling.transport.http

import storage.controlplane.scaling.service.ControllerService
import storage.controlplane.scaling.transport.dto.ControlDecisionResponse
import zio.{TaskLayer, UIO, URLayer, ZLayer}

final class ScalingDecisionHttpHandler(controller: ControllerService) {
  def current: UIO[ControlDecisionResponse] = controller.snapshot.map(ControlDecisionResponse.fromDomain)
}

object ScalingDecisionHttpHandler {
  val layer: URLayer[ControllerService, ScalingDecisionHttpHandler] = ZLayer.derive[ScalingDecisionHttpHandler]
  val module: TaskLayer[ScalingDecisionHttpHandler] =
    ZLayer.make[ScalingDecisionHttpHandler](ControllerService.module, layer)
}
