package storage.controlplane.systemstate.client.prometheus

enum PrometheusMetricsProfile(val argument: String) {
  case Docker extends PrometheusMetricsProfile("docker")
  case Kubernetes extends PrometheusMetricsProfile("kubernetes")
}

object PrometheusMetricsProfile {
  def parse(value: String): Either[String, PrometheusMetricsProfile] =
    PrometheusMetricsProfile.values
      .find(_.argument == value)
      .toRight(s"unsupported Prometheus metrics profile: $value")
}
