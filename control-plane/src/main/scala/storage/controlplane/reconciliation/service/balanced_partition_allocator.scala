package storage.controlplane.reconciliation.service

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import storage.controlplane.reconciliation.domain.AllocationError
import storage.controlplane.reconciliation.domain.AllocationError.InvalidInput
import zio.{TaskLayer, ZLayer}

final class BalancedPartitionAllocator extends PartitionAllocator {
  override def allocate(
      current: AssignmentSnapshot,
      targetMembers: Set[Node]
  ): Either[AllocationError, AssignmentSnapshot] =
    for {
      validCurrent <- current.validate.left.map(InvalidInput.apply)
      orderedMembers = targetMembers.toList.sortBy(_.id)
      _ <- Either.cond(orderedMembers.nonEmpty, (), InvalidInput("target members must not be empty"))
      _ <- Either.cond(
        orderedMembers.map(_.id).distinct.size == orderedMembers.size,
        (),
        InvalidInput("target member identifiers must be unique")
      )
      capacities = targetCapacities(validCurrent.partitionCount, orderedMembers)
      retained = retainExistingOwners(validCurrent, capacities)
      owners <- fillUnassigned(orderedMembers, capacities, retained)
      snapshot = AssignmentSnapshot(
        validCurrent.epoch + 1,
        validCurrent.partitionCount,
        orderedMembers,
        owners.zipWithIndex.map { case (nodeId, partition) => PartitionOwner(partition, nodeId) }.toList
      )
      valid <- snapshot.validate.left.map(InvalidInput.apply)
    } yield valid

  private def targetCapacities(partitionCount: Int, members: List[Node]): Map[String, Int] = {
    val base = partitionCount / members.size
    val remainder = partitionCount % members.size
    members.zipWithIndex.map { case (member, index) =>
      member.id -> (base + (if (index < remainder) 1 else 0))
    }.toMap
  }

  private def retainExistingOwners(
      current: AssignmentSnapshot,
      capacities: Map[String, Int]
  ): Vector[Option[String]] = {
    val emptyCounts = capacities.keys.map(_ -> 0).toMap
    current.assignments
      .sortBy(_.partition)
      .foldLeft((Vector.empty[Option[String]], emptyCounts)) { case ((retained, retainedCounts), owner) =>
        capacities.get(owner.nodeId) match {
          case Some(capacity) if retainedCounts(owner.nodeId) < capacity =>
            (
              retained :+ Some(owner.nodeId),
              retainedCounts.updated(owner.nodeId, retainedCounts(owner.nodeId) + 1)
            )
          case _ => (retained :+ None, retainedCounts)
        }
      }
      ._1
  }

  private def fillUnassigned(
      members: List[Node],
      capacities: Map[String, Int],
      retained: Vector[Option[String]]
  ): Either[AllocationError, Vector[String]] = {
    val assignedCounts = retained.flatten.groupMapReduce(identity)(_ => 1)(_ + _)
    val available =
      members.flatMap(member => List.fill(capacities(member.id) - assignedCounts.getOrElse(member.id, 0))(member.id))
    retained
      .foldLeft[Either[AllocationError, (Vector[String], List[String])]](Right(Vector.empty -> available)) {
        case (result, retainedOwner) =>
          result.flatMap { case (owners, remaining) =>
            retainedOwner match {
              case Some(nodeId)               => Right((owners :+ nodeId) -> remaining)
              case None if remaining.nonEmpty => Right((owners :+ remaining.head) -> remaining.tail)
              case None                       => Left(InvalidInput("allocator capacity does not cover every partition"))
            }
          }
      }
      .flatMap { case (owners, remaining) =>
        Either.cond(remaining.isEmpty, owners, InvalidInput("allocator capacity exceeds partition count"))
      }
  }
}

object BalancedPartitionAllocator {
  val layer: TaskLayer[PartitionAllocator] = ZLayer.succeed(BalancedPartitionAllocator())
  val module: TaskLayer[PartitionAllocator] = layer
}
