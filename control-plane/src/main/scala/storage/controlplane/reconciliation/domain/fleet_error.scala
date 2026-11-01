package storage.controlplane.reconciliation.domain

sealed abstract class FleetError(message: String, cause: Throwable = null) extends Exception(message, cause)

object FleetError {
  final case class InvalidState(details: String) extends FleetError(details)
  final case class Conflict(details: String) extends FleetError(details)
  final case class Forbidden(details: String) extends FleetError(details)
  final case class Unavailable(underlying: Throwable)
      extends FleetError("processing unit fleet unavailable", underlying)
}
