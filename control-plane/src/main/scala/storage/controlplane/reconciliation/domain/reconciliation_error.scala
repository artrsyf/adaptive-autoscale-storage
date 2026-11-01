package storage.controlplane.reconciliation.domain

sealed abstract class ReconciliationError(message: String, cause: Throwable = null) extends Exception(message, cause)

object ReconciliationError {
  final case class InvalidDecision(details: String) extends ReconciliationError(details)
  final case class InvalidOperation(details: String) extends ReconciliationError(details)
  case object OperationConflict extends ReconciliationError("scaling operation conflict")
  final case class DependencyFailure(dependency: String, underlying: Throwable)
      extends ReconciliationError(s"$dependency unavailable", underlying)
  final case class Unavailable(underlying: Throwable)
      extends ReconciliationError("scaling operation repository unavailable", underlying)
}
