package storage.controlplane.reconciliation.service

import storage.controlplane.reconciliation.domain.{ReconciliationError, ScalingOperation}
import zio.IO

trait ScalingOperationRepository {
  def active: IO[ReconciliationError, Option[ScalingOperation]]
  def latest: IO[ReconciliationError, Option[ScalingOperation]]
  def create(operation: ScalingOperation): IO[ReconciliationError, ScalingOperation]
  def save(expectedVersion: Long, operation: ScalingOperation): IO[ReconciliationError, ScalingOperation]
}
