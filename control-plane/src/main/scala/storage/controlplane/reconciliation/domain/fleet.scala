package storage.controlplane.reconciliation.domain

import storage.controlplane.assignment.domain.Node

final case class ReplicaCount(value: Int) extends AnyVal

final case class FleetSnapshot(
    desiredReplicas: ReplicaCount,
    observedReplicas: ReplicaCount,
    readyMembers: Set[Node]
) {
  def validate: Either[String, FleetSnapshot] =
    if (desiredReplicas.value < 0 || observedReplicas.value < 0)
      Left("replica counts must be non-negative")
    else if (readyMembers.size > observedReplicas.value)
      Left("ready members cannot exceed observed replicas")
    else if (readyMembers.map(_.id).size != readyMembers.size)
      Left("ready member identifiers must be unique")
    else Right(this)
}
