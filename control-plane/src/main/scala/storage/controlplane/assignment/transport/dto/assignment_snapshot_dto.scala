package storage.controlplane.assignment.transport.dto

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import zio.json.{jsonField, jsonNoExtraFields, DeriveJsonCodec, JsonCodec}

@jsonNoExtraFields
final case class AssignmentNodeDto(id: String, url: String)
object AssignmentNodeDto {
  implicit val codec: JsonCodec[AssignmentNodeDto] = DeriveJsonCodec.gen[AssignmentNodeDto]
}

@jsonNoExtraFields
final case class PartitionAssignmentDto(partition: Int, @jsonField("node_id") nodeId: String)
object PartitionAssignmentDto {
  implicit val codec: JsonCodec[PartitionAssignmentDto] = DeriveJsonCodec.gen[PartitionAssignmentDto]
}

@jsonNoExtraFields
final case class AssignmentSnapshotDto(
    epoch: Long,
    @jsonField("partition_count") partitionCount: Int,
    nodes: List[AssignmentNodeDto],
    assignments: List[PartitionAssignmentDto]
) {
  def toDomain: AssignmentSnapshot =
    AssignmentSnapshot(
      epoch,
      partitionCount,
      nodes.map(node => Node(node.id, node.url)),
      assignments.map(owner => PartitionOwner(owner.partition, owner.nodeId))
    )
}

object AssignmentSnapshotDto {
  implicit val codec: JsonCodec[AssignmentSnapshotDto] = DeriveJsonCodec.gen[AssignmentSnapshotDto]

  def fromDomain(snapshot: AssignmentSnapshot): AssignmentSnapshotDto =
    AssignmentSnapshotDto(
      snapshot.epoch,
      snapshot.partitionCount,
      snapshot.nodes.map(node => AssignmentNodeDto(node.id, node.url)),
      snapshot.assignments.map(owner => PartitionAssignmentDto(owner.partition, owner.nodeId))
    )
}
