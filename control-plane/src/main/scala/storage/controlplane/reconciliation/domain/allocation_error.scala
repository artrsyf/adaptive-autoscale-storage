package storage.controlplane.reconciliation.domain

sealed abstract class AllocationError(message: String) extends Exception(message)

object AllocationError {
  final case class InvalidInput(details: String) extends AllocationError(details)
}
