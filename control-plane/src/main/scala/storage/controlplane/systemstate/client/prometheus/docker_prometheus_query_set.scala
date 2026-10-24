package storage.controlplane.systemstate.client.prometheus

import zio.{ULayer, ZLayer}

object DockerPrometheusQuerySet extends PrometheusQuerySet {
  val layer: ULayer[PrometheusQuerySet] = ZLayer.succeed(this)
  val module: ULayer[PrometheusQuerySet] = layer

  override protected def cpuUtilization(window: String): String =
    resourceUtilization(
      s"sum by(container_label_com_docker_compose_service)(rate(container_cpu_usage_seconds_total{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\",image!=\"\"}[$window]))",
      "max by(container_label_com_docker_compose_service)(container_spec_cpu_quota{container_label_com_docker_compose_project=\"autoscale-storage\"} / container_spec_cpu_period{container_label_com_docker_compose_project=\"autoscale-storage\"})"
    )

  override protected def memoryUtilization: String =
    resourceUtilization(
      "max by(container_label_com_docker_compose_service)(container_memory_working_set_bytes{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\",image!=\"\"})",
      "max by(container_label_com_docker_compose_service)(container_spec_memory_limit_bytes{container_label_com_docker_compose_project=\"autoscale-storage\",container_label_com_docker_compose_service=~\"processing-unit-.+\"})"
    )

  private def resourceUtilization(numerator: String, denominator: String): String =
    s"label_replace(($numerator) / on(container_label_com_docker_compose_service) ($denominator), \"node\", \"$$1\", \"container_label_com_docker_compose_service\", \"(.*)\")"
}
