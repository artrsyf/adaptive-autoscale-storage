package storage.controlplane.systemstate.transport.http

import sttp.tapir.ztapir._
import sttp.tapir.json.zio._
import storage.controlplane.systemstate.transport.dto.ControlStateResponse
import zio.{Chunk, TaskLayer, URLayer, ZLayer}

final class SystemStateHttpRoutes(handler: SystemStateHttpHandler) {
  private val current = endpoint.get
    .in("control" / "state")
    .out(jsonBody[ControlStateResponse])
    .zServerLogic[Any](_ => handler.current)

  val endpoints: Chunk[ZServerEndpoint[Any, Any]] = Chunk(current)
}

object SystemStateHttpRoutes {
  val layer: URLayer[SystemStateHttpHandler, SystemStateHttpRoutes] = ZLayer.derive[SystemStateHttpRoutes]
  val module: TaskLayer[SystemStateHttpRoutes] =
    ZLayer.make[SystemStateHttpRoutes](SystemStateHttpHandler.module, layer)
}
