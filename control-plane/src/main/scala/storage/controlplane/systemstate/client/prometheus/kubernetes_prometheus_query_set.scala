package storage.controlplane.systemstate.client.prometheus

import zio.{ULayer, ZLayer}

object KubernetesPrometheusQuerySet extends PrometheusQuerySet {
  val layer: ULayer[PrometheusQuerySet] = ZLayer.succeed(this)
  val module: ULayer[PrometheusQuerySet] = layer

  override protected def cpuUtilization(window: String): String =
    resourceUtilization(
      s"sum by(pod)(rate(container_cpu_usage_seconds_total{namespace=\"autoscale-storage\",pod=~\"processing-unit-[0-9]+\",container=\"processing-unit\",image!=\"\"}[$window]))",
      "max by(pod)(kube_pod_container_resource_limits{namespace=\"autoscale-storage\",pod=~\"processing-unit-[0-9]+\",container=\"processing-unit\",resource=\"cpu\",unit=\"core\"})"
    )

  override protected def memoryUtilization: String =
    resourceUtilization(
      "max by(pod)(container_memory_working_set_bytes{namespace=\"autoscale-storage\",pod=~\"processing-unit-[0-9]+\",container=\"processing-unit\",image!=\"\"})",
      "max by(pod)(kube_pod_container_resource_limits{namespace=\"autoscale-storage\",pod=~\"processing-unit-[0-9]+\",container=\"processing-unit\",resource=\"memory\",unit=\"byte\"})"
    )

  private def resourceUtilization(numerator: String, denominator: String): String =
    s"label_replace(($numerator) / on(pod) ($denominator), \"node\", \"$$1\", \"pod\", \"(.*)\")"
}
