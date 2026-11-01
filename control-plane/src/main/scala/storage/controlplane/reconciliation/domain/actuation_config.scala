package storage.controlplane.reconciliation.domain

import java.net.URI

import storage.controlplane.config.ControlPlaneConfig
import zio.{TaskLayer, URLayer, ZLayer}

import scala.concurrent.duration.FiniteDuration

enum ActuationMode(val wireName: String) {
  case Advisory extends ActuationMode("advisory")
  case Kubernetes extends ActuationMode("kubernetes")
}

object ActuationMode {
  def parse(value: String): Either[String, ActuationMode] =
    values.find(_.wireName == value).toRight(s"unknown actuation mode: $value")
}

final case class ActuationConfig(
    mode: ActuationMode,
    reconcileInterval: FiniteDuration,
    requestTimeout: FiniteDuration,
    operationTimeout: FiniteDuration,
    routerUrl: String,
    namespace: String,
    workloadName: String,
    headlessService: String,
    applicationPort: Int
) {
  def validate: Either[String, ActuationConfig] =
    if (
      reconcileInterval.length <= 0 || requestTimeout.length <= 0 || operationTimeout.length <= 0 ||
      operationTimeout < reconcileInterval
    ) Left("invalid actuation timeouts")
    else if (!validHttpOrigin(routerUrl)) Left("invalid Router URL")
    else if (
      namespace.trim.isEmpty || workloadName.trim.isEmpty || headlessService.trim.isEmpty ||
      applicationPort < 1 || applicationPort > 65535
    ) Left("invalid processing unit fleet configuration")
    else Right(this)

  private def validHttpOrigin(value: String): Boolean =
    try {
      val uri = URI.create(value)
      Set("http", "https").contains(uri.getScheme) && uri.getHost != null &&
      Option(uri.getPath).forall(path => path.isEmpty || path == "/") &&
      uri.getQuery == null && uri.getFragment == null
    } catch {
      case _: IllegalArgumentException => false
    }
}

object ActuationConfig {
  val layer: URLayer[ControlPlaneConfig, ActuationConfig] =
    ZLayer.fromFunction((config: ControlPlaneConfig) => config.actuation)
  val module: TaskLayer[ActuationConfig] = ZLayer.make[ActuationConfig](ControlPlaneConfig.module, layer)
}
