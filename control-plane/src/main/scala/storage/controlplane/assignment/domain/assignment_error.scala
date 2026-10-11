package storage.controlplane.assignment.domain

sealed abstract class AssignmentError(message: String, cause: Throwable = null) extends RuntimeException(message, cause)

object AssignmentError {
  final case class InvalidAssignment(details: String) extends AssignmentError(details)
  case object EpochConflict extends AssignmentError("epoch_conflict")
  final case class Unavailable(underlying: Throwable) extends AssignmentError("assignment_unavailable", underlying)
}
