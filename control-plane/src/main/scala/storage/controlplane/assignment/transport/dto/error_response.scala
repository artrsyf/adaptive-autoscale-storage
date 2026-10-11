package storage.controlplane.assignment.transport.dto

import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonNoExtraFields, JsonCodec}

@jsonNoExtraFields
final case class ErrorResponse(error: String) derives JsonCodec, Loggable, CanEqual, Schema
