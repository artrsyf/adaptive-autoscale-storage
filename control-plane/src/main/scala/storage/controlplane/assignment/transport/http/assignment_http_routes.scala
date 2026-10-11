package storage.controlplane.assignment.transport.http

import sttp.model.StatusCode
import sttp.tapir.generic.auto._
import sttp.tapir.json.zio._
import sttp.tapir.server.model.EndpointExtensions._
import sttp.tapir.ztapir._
import storage.controlplane.assignment.transport.dto.{AssignmentPublishRequest, AssignmentSnapshotDto, ErrorResponse}
import storage.controlplane.config.ApiConfig
import zio.json._
import zio.{Chunk, ZIO}

final class AssignmentHttpRoutes(handler: AssignmentHttpHandler, config: ApiConfig) {
  private val baseEndpoint = endpoint.in("assignments" / "current")

  private val getCurrent = baseEndpoint.get
    .out(jsonBody[AssignmentSnapshotDto])
    .errorOut(statusCode.and(jsonBody[ErrorResponse]))
    .zServerLogic[Any](_ => handler.getCurrent)

  private val publish = baseEndpoint.put
    .maxRequestBodyLength(config.maxBodyBytes)
    .in(stringJsonBody)
    .out(jsonBody[AssignmentSnapshotDto])
    .errorOut(statusCode.and(jsonBody[ErrorResponse]))
    .zServerLogic[Any](body =>
      ZIO
        .fromEither(body.fromJson[AssignmentPublishRequest])
        .mapError(_ => StatusCode.BadRequest -> ErrorResponse("invalid_request"))
        .flatMap(handler.publish)
    )

  val endpoints: Chunk[ZServerEndpoint[Any, Any]] = Chunk(getCurrent, publish)
}
