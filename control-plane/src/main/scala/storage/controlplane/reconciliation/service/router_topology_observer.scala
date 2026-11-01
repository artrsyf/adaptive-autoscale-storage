package storage.controlplane.reconciliation.service

import storage.controlplane.reconciliation.domain.{RouterTopologyError, RouterTopologyState}
import zio.IO

trait RouterTopologyObserver {
  def current: IO[RouterTopologyError, RouterTopologyState]
}
