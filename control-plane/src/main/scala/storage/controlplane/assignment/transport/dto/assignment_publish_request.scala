package storage.controlplane.assignment.transport.dto

import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonMemberNames, jsonNoExtraFields, JsonCodec, SnakeCase}

@jsonNoExtraFields
@jsonMemberNames(SnakeCase)
final case class AssignmentPublishRequest(
    expectedEpoch: Long,
    assignment: AssignmentSnapshotDto
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
