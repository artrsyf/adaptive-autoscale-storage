package storage.controlplane.assignment.transport.dto

import zio.json.{jsonField, jsonNoExtraFields, DeriveJsonCodec, JsonCodec}

@jsonNoExtraFields
final case class AssignmentPublishRequest(
    @jsonField("expected_epoch") expectedEpoch: Long,
    assignment: AssignmentSnapshotDto
)

object AssignmentPublishRequest {
  implicit val codec: JsonCodec[AssignmentPublishRequest] = DeriveJsonCodec.gen[AssignmentPublishRequest]
}
