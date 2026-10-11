package storage.controlplane.assignment.transport.http

import sttp.tapir.json.zio._
import sttp.tapir.server.model.EndpointExtensions._
import sttp.tapir.ztapir._
import storage.controlplane.assignment.transport.dto.{AssignmentPublishRequest, AssignmentSnapshotDto, ErrorResponse}
import storage.controlplane.config.ApiConfig
import zio.{Chunk, TaskLayer, URLayer, ZLayer}

final class AssignmentHttpRoutes(handler: AssignmentHttpHandler, config: ApiConfig) {
  private val baseEndpoint = endpoint.in("assignments" / "current")

  private val getCurrent = baseEndpoint.get
    .out(jsonBody[AssignmentSnapshotDto])
    .errorOut(statusCode.and(jsonBody[ErrorResponse]))
    .zServerLogic[Any](_ => handler.getCurrent)

  private val publish = baseEndpoint.put
    .maxRequestBodyLength(config.maxBodyBytes)
    .in(jsonBody[AssignmentPublishRequest])
    .out(jsonBody[AssignmentSnapshotDto])
    .errorOut(statusCode.and(jsonBody[ErrorResponse]))
    .zServerLogic[Any](handler.publish)

  val endpoints: Chunk[ZServerEndpoint[Any, Any]] = Chunk(getCurrent, publish)
}

object AssignmentHttpRoutes {
  val layer: URLayer[AssignmentHttpHandler & ApiConfig, AssignmentHttpRoutes] = ZLayer.derive[AssignmentHttpRoutes]
  val module: TaskLayer[AssignmentHttpRoutes] =
    ZLayer.make[AssignmentHttpRoutes](AssignmentHttpHandler.module, ApiConfig.module, layer)
}
