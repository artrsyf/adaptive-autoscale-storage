package storage.controlplane.systemstate.transport.http

import storage.controlplane.scaling.service.ControllerService
import storage.controlplane.systemstate.transport.dto.ControlStateResponse
import zio.{TaskLayer, UIO, URLayer, ZLayer}

final class SystemStateHttpHandler(controller: ControllerService) {
  def current: UIO[ControlStateResponse] = controller.snapshot.map(ControlStateResponse.fromDomain)
}

object SystemStateHttpHandler {
  val layer: URLayer[ControllerService, SystemStateHttpHandler] = ZLayer.derive[SystemStateHttpHandler]
  val module: TaskLayer[SystemStateHttpHandler] =
    ZLayer.make[SystemStateHttpHandler](ControllerService.module, layer)
}
