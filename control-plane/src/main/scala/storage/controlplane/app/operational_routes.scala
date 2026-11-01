package storage.controlplane.app

import storage.controlplane.assignment.service.{AssignmentRepository, AssignmentService}
import storage.controlplane.assignment.transport.dto.ErrorResponse
import storage.controlplane.config.ServerConfig
import storage.controlplane.reconciliation.client.advisory.AdvisoryProcessingUnitFleet
import storage.controlplane.reconciliation.service.{ProcessingUnitFleet, ScalingOperationService}
import storage.controlplane.scaling.domain.ScalingConfig
import storage.controlplane.scaling.service.ControllerService
import zio.http.{Handler, Method, Response, Routes, Status}
import zio.json._
import storage.controlplane.assignment.repository.postgres.AssignmentPostgresRepository
import zio.{Clock, Duration, TaskLayer, URLayer, ZIO, ZLayer}

final class OperationalRoutes(
    repository: AssignmentRepository,
    service: AssignmentService,
    controller: ControllerService,
    operations: ScalingOperationService,
    fleet: ProcessingUnitFleet,
    scalingConfig: ScalingConfig,
    config: ServerConfig
) {
  val routes: Routes[Any, Response] =
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
        for {
          epoch <- service.epoch
          snapshot <- controller.snapshot
          latestOperation <- operations.latest.orDie
          observedFleet <- fleet.observe.option
          now <- Clock.instant
        } yield Response.text(
          ControllerMetrics.render(snapshot, scalingConfig, now, epoch, latestOperation, observedFleet)
        )
      }
    )
}

object OperationalRoutes {
  val layer: URLayer[
    AssignmentRepository & AssignmentService & ControllerService & ScalingOperationService & ProcessingUnitFleet &
      ScalingConfig & ServerConfig,
    OperationalRoutes
  ] = ZLayer.derive[OperationalRoutes]

  val module: TaskLayer[OperationalRoutes] =
    ZLayer.make[OperationalRoutes](
      AssignmentPostgresRepository.module,
      AssignmentService.module,
      ControllerService.module,
      ScalingOperationService.module,
      AdvisoryProcessingUnitFleet.module,
      ScalingConfig.module,
      ServerConfig.module,
      layer
    )
}
