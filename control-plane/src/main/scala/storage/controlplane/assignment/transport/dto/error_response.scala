package storage.controlplane.assignment.transport.dto

import zio.json.{jsonNoExtraFields, DeriveJsonCodec, JsonCodec}

@jsonNoExtraFields
final case class ErrorResponse(error: String)

object ErrorResponse {
  implicit val codec: JsonCodec[ErrorResponse] = DeriveJsonCodec.gen[ErrorResponse]
}
