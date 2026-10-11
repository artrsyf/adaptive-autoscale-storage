package storage.controlplane.assignment.domain

import zio.test._

object AssignmentSpec extends ZIOSpecDefault {
  private val valid = AssignmentSnapshot(
    epoch = 1,
    partitionCount = 2,
    nodes = List(Node("one", "http://one:8080"), Node("two", "http://two:8080")),
    assignments = List(PartitionOwner(0, "one"), PartitionOwner(1, "two"))
  )

  override def spec = suite("AssignmentSnapshot")(
    test("accepts a complete snapshot") {
      assertTrue(valid.validate == Right(valid))
    },
    test("rejects invalid dimensions, nodes and owners") {
      val invalid = List(
        valid.copy(epoch = 0),
        valid.copy(partitionCount = 3),
        valid.copy(nodes = valid.nodes :+ valid.nodes.head),
        valid.copy(nodes = List(Node("one", "https://one:8080"))),
        valid.copy(assignments = List(PartitionOwner(0, "one"), PartitionOwner(0, "one"))),
        valid.copy(assignments = List(PartitionOwner(0, "one"), PartitionOwner(1, "missing")))
      )
      assertTrue(invalid.forall(_.validate.isLeft))
    }
  )
}
