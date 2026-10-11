package storage.controlplane.systemstate.client.prometheus

import zio.json.JsonDecoder

private[prometheus] final case class PrometheusVectorSample(metric: Map[String, String], value: (Double, String))
    derives JsonDecoder,
      CanEqual

private[prometheus] final case class PrometheusData(resultType: String, result: List[PrometheusVectorSample])
    derives JsonDecoder,
      CanEqual

private[prometheus] final case class PrometheusResponse(
    status: String,
    data: Option[PrometheusData],
    errorType: Option[String],
    error: Option[String]
) derives JsonDecoder,
      CanEqual
