package storage.controlplane.app

import storage.controlplane.assignment.service.{AssignmentRepository, AssignmentService}
import storage.controlplane.assignment.transport.dto.ErrorResponse
import storage.controlplane.config.ServerConfig
import zio.http.{Handler, Method, Response, Routes, Status}
import zio.json._
import zio.{Duration, ZIO}

object OperationalRoutes {
  def make(
      repository: AssignmentRepository,
      service: AssignmentService,
      config: ServerConfig
  ): Routes[Any, Response] =
    Routes(
      Method.GET / "livez" -> Handler.fromResponse(Response.status(Status.Ok)),
      Method.GET / "readyz" -> Handler.fromZIO {
        repository.ready
          .timeoutFail(new RuntimeException("readiness timed out"))(Duration.fromMillis(config.readyTimeout.toMillis))
          .as(Response.status(Status.Ok))
          .catchAll(_ =>
            ZIO.succeed(
              Response
                .json(ErrorResponse("not_ready").toJson)
                .status(Status.ServiceUnavailable)
            )
          )
      },
      Method.GET / "metrics" -> Handler.fromZIO {
        service.epoch.map { epoch =>
          Response.text(
            "# HELP storage_assignment_epoch Active assignment epoch.\n" +
              "# TYPE storage_assignment_epoch gauge\n" +
              s"storage_assignment_epoch $epoch\n"
          )
        }
      }
    )
}
