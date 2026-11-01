package storage.controlplane.reconciliation.client.router.http

import zio.json.{jsonMemberNames, jsonNoExtraFields, JsonDecoder, SnakeCase}

@jsonNoExtraFields
@jsonMemberNames(SnakeCase)
private[http] final case class RouterTopologyResponse(
    epoch: Long,
    partitionCount: Int,
    nodeIds: List[String]
) derives JsonDecoder,
      CanEqual
