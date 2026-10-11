package storage.controlplane.scaling.transport.http

import sttp.tapir.ztapir._
import sttp.tapir.json.zio._
import storage.controlplane.scaling.transport.dto.ControlDecisionResponse
import zio.{Chunk, TaskLayer, URLayer, ZLayer}

final class ScalingDecisionHttpRoutes(handler: ScalingDecisionHttpHandler) {
  private val current = endpoint.get
    .in("control" / "decision")
    .out(jsonBody[ControlDecisionResponse])
    .zServerLogic[Any](_ => handler.current)

  val endpoints: Chunk[ZServerEndpoint[Any, Any]] = Chunk(current)
}

object ScalingDecisionHttpRoutes {
  val layer: URLayer[ScalingDecisionHttpHandler, ScalingDecisionHttpRoutes] = ZLayer.derive[ScalingDecisionHttpRoutes]
  val module: TaskLayer[ScalingDecisionHttpRoutes] =
    ZLayer.make[ScalingDecisionHttpRoutes](ScalingDecisionHttpHandler.module, layer)
}
