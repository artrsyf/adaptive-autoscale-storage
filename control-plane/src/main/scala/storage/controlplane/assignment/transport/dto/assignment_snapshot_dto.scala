package storage.controlplane.assignment.transport.dto

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonMemberNames, jsonNoExtraFields, JsonCodec, SnakeCase}

@jsonNoExtraFields
@jsonMemberNames(SnakeCase)
final case class AssignmentNodeDto(id: String, url: String) derives JsonCodec, Loggable, CanEqual, Schema

@jsonNoExtraFields
@jsonMemberNames(SnakeCase)
final case class PartitionAssignmentDto(partition: Int, nodeId: String) derives JsonCodec, Loggable, CanEqual, Schema

@jsonNoExtraFields
@jsonMemberNames(SnakeCase)
final case class AssignmentSnapshotDto(
    epoch: Long,
    partitionCount: Int,
    nodes: List[AssignmentNodeDto],
    assignments: List[PartitionAssignmentDto]
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema {
  def toDomain: AssignmentSnapshot =
    AssignmentSnapshot(
      epoch,
      partitionCount,
      nodes.map(node => Node(node.id, node.url)),
      assignments.map(owner => PartitionOwner(owner.partition, owner.nodeId))
    )
}

object AssignmentSnapshotDto {
  def fromDomain(snapshot: AssignmentSnapshot): AssignmentSnapshotDto =
    AssignmentSnapshotDto(
      snapshot.epoch,
      snapshot.partitionCount,
      snapshot.nodes.map(node => AssignmentNodeDto(node.id, node.url)),
      snapshot.assignments.map(owner => PartitionAssignmentDto(owner.partition, owner.nodeId))
    )
}
