package storage.controlplane.assignment.domain

import java.net.URI

final case class Node(id: String, url: String)

final case class PartitionOwner(partition: Int, nodeId: String)

final case class AssignmentSnapshot(
    epoch: Long,
    partitionCount: Int,
    nodes: List[Node],
    assignments: List[PartitionOwner]
) {
  def validate: Either[String, AssignmentSnapshot] = {
    val nodeIds = nodes.map(_.id)
    val partitions = assignments.map(_.partition)
    if (epoch <= 0 || partitionCount < 1 || partitionCount > 1024 || nodes.isEmpty)
      Left("invalid epoch, partition count or nodes")
    else if (assignments.size != partitionCount) Left("assignment must contain every partition")
    else if (
      nodeIds.exists(_.trim.isEmpty) || nodeIds.distinct.size != nodeIds.size || nodes
        .exists(node => !validOrigin(node.url))
    )
      Left("invalid or duplicate node")
    else if (
      partitions.distinct.size != partitions.size ||
      partitions.exists(partition => partition < 0 || partition >= partitionCount) ||
      assignments.exists(owner => !nodeIds.contains(owner.nodeId))
    ) Left("invalid, duplicate or unowned partition")
    else Right(this)
  }

  private def validOrigin(value: String): Boolean =
    try {
      val uri = URI.create(value)
      uri.getScheme == "http" && uri.getHost != null && uri.getUserInfo == null &&
      Option(uri.getPath).forall(path => path.isEmpty || path == "/") && uri.getQuery == null && uri.getFragment == null
    } catch {
      case _: IllegalArgumentException => false
    }
}
