package storage.controlplane.assignment.transport.dto

import zio.json._
import zio.test._

object AssignmentPublishRequestSpec extends ZIOSpecDefault {
  private val validJson =
    """{"expected_epoch":1,"assignment":{"epoch":2,"partition_count":1,"nodes":[{"id":"one","url":"http://one:8080"}],"assignments":[{"partition":0,"node_id":"one"}]}}"""

  override def spec = suite("AssignmentPublishRequest")(
    test("decodes the existing wire contract") {
      val decoded = validJson.fromJson[AssignmentPublishRequest]
      assertTrue(
        decoded.exists(_.expectedEpoch == 1),
        decoded.exists(_.assignment.partitionCount == 1)
      )
    },
    test("rejects unknown JSON fields") {
      val withUnknownField = validJson.dropRight(1) + ",\"unexpected\":true}"
      assertTrue(withUnknownField.fromJson[AssignmentPublishRequest].isLeft)
    }
  )
}
