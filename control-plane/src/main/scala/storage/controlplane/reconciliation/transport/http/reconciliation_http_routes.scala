package storage.controlplane.reconciliation.transport.http

import sttp.tapir.json.zio._
import sttp.tapir.ztapir._
import storage.controlplane.reconciliation.transport.dto.ReconciliationResponse
import zio.{Chunk, TaskLayer, URLayer, ZLayer}

final class ReconciliationHttpRoutes(handler: ReconciliationHttpHandler) {
  private val current = endpoint.get
    .in("control" / "reconciliation")
    .out(jsonBody[ReconciliationResponse])
    .zServerLogic[Any](_ => handler.current)

  val endpoints: Chunk[ZServerEndpoint[Any, Any]] = Chunk(current)
}

object ReconciliationHttpRoutes {
  val layer: URLayer[ReconciliationHttpHandler, ReconciliationHttpRoutes] = ZLayer.derive[ReconciliationHttpRoutes]
  val module: TaskLayer[ReconciliationHttpRoutes] =
    ZLayer.make[ReconciliationHttpRoutes](ReconciliationHttpHandler.module, layer)
}
