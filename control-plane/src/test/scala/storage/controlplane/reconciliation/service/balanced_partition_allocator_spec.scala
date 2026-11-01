package storage.controlplane.reconciliation.service

import storage.controlplane.assignment.domain.{AssignmentSnapshot, Node, PartitionOwner}
import zio.test._

object BalancedPartitionAllocatorSpec extends ZIOSpecDefault {
  private val allocator = BalancedPartitionAllocator()

  override def spec = suite("BalancedPartitionAllocator")(
    test("keeps a complete balanced assignment and increments epoch") {
      val current = assignment(3)
      val result = allocator.allocate(current, nodes(5)).toOption.get
      val counts = result.assignments.groupMapReduce(_.nodeId)(_ => 1)(_ + _).values.toList
      val moved = result.assignments.zip(current.assignments).count { case (next, previous) =>
        next.nodeId != previous.nodeId
      }
      assertTrue(
        result.validate.isRight,
        result.epoch == current.epoch + 1,
        counts.max - counts.min <= 1,
        moved == 50
      )
    },
    test("removes departed members with no unnecessary movement") {
      val current = allocator.allocate(assignment(3), nodes(5)).toOption.get
      val result = allocator.allocate(current, nodes(3)).toOption.get
      val moved = result.assignments.zip(current.assignments).count { case (next, previous) =>
        next.nodeId != previous.nodeId
      }
      assertTrue(
        result.nodes.map(_.id).toSet == Set("processing-unit-1", "processing-unit-2", "processing-unit-3"),
        moved == 50,
        result.validate.isRight
      )
    },
    test("is deterministic and rejects an empty target fleet") {
      val current = assignment(3)
      val first = allocator.allocate(current, nodes(4))
      val second = allocator.allocate(current, nodes(4).toList.reverse.toSet)
      assertTrue(first == second, allocator.allocate(current, Set.empty).isLeft)
    }
  )

  private def nodes(count: Int): Set[Node] =
    (1 to count).map(index => Node(s"processing-unit-$index", s"http://processing-unit-$index:8080")).toSet

  private def assignment(nodeCount: Int): AssignmentSnapshot = {
    val members = nodes(nodeCount).toList.sortBy(_.id)
    AssignmentSnapshot(
      7,
      128,
      members,
      (0 until 128).map(partition => PartitionOwner(partition, members(partition % members.size).id)).toList
    )
  }
}
