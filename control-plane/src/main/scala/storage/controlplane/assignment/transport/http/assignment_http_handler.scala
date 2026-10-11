package storage.controlplane.assignment.transport.http

import java.util.concurrent.TimeoutException

import sttp.model.StatusCode
import storage.controlplane.assignment.domain.AssignmentError.{EpochConflict, InvalidAssignment, Unavailable}
import storage.controlplane.assignment.domain.AssignmentError
import storage.controlplane.assignment.service.AssignmentService
import storage.controlplane.assignment.transport.dto.{AssignmentPublishRequest, AssignmentSnapshotDto, ErrorResponse}
import storage.controlplane.config.ApiConfig
import zio.{Duration, IO}

final class AssignmentHttpHandler(service: AssignmentService, config: ApiConfig) {
  type ApiFailure = (StatusCode, ErrorResponse)

  def getCurrent: IO[ApiFailure, AssignmentSnapshotDto] =
    withinTimeout(service.current).map(AssignmentSnapshotDto.fromDomain).mapError(toFailure)

  def publish(request: AssignmentPublishRequest): IO[ApiFailure, AssignmentSnapshotDto] =
    withinTimeout(service.publish(request.expectedEpoch, request.assignment.toDomain))
      .map(AssignmentSnapshotDto.fromDomain)
      .mapError(toFailure)

  private def withinTimeout[A](effect: IO[AssignmentError, A]): IO[AssignmentError, A] =
    effect.timeoutFail(Unavailable(new TimeoutException("assignment request timed out")))(
      Duration.fromMillis(config.requestTimeout.toMillis)
    )

  private def toFailure(error: AssignmentError): ApiFailure = error match {
    case _: InvalidAssignment => StatusCode.BadRequest -> ErrorResponse("invalid_assignment")
    case EpochConflict        => StatusCode.Conflict -> ErrorResponse("epoch_conflict")
    case _: Unavailable       => StatusCode.ServiceUnavailable -> ErrorResponse("assignment_unavailable")
  }
}
