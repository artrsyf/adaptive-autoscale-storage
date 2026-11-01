package storage.controlplane.reconciliation.service

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node}
import storage.controlplane.reconciliation.domain.AllocationError

trait PartitionAllocator {
  def allocate(current: AssignmentSnapshot, targetMembers: Set[Node]): Either[AllocationError, AssignmentSnapshot]
}
