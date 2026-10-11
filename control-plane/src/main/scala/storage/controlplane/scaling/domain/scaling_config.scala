package storage.controlplane.scaling.domain

import java.time.Duration

import storage.controlplane.systemstate.domain.SignalName
import storage.controlplane.config.ControlPlaneConfig
import zio.{TaskLayer, URLayer, ZLayer}

final case class ThresholdBand(lower: Double, upper: Double) {
  def validate: Either[String, ThresholdBand] =
    if (!lower.isFinite || !upper.isFinite || lower < 0 || upper <= lower)
      Left("thresholds must be finite, non-negative and upper must exceed lower")
    else Right(this)
}

final case class ScalingConfig(
    minReplicas: Int,
    maxReplicas: Int,
    step: Int,
    confirmationWindows: Int,
    cooldown: Duration,
    thresholds: Map[SignalName, ThresholdBand]
) {
  def validate: Either[String, ScalingConfig] =
    if (minReplicas < 1 || maxReplicas < minReplicas || step < 1 || confirmationWindows < 1 || cooldown.isNegative)
      Left("invalid replica, confirmation or cooldown configuration")
    else if (thresholds.keySet != SignalName.policySignals) Left("thresholds must cover every policy signal")
    else
      thresholds.values
        .foldLeft[Either[String, Unit]](Right(()))((result, band) => result.flatMap(_ => band.validate.map(_ => ())))
        .map(_ => this)
}

object ScalingConfig {
  val layer: URLayer[ControlPlaneConfig, ScalingConfig] =
    ZLayer.fromFunction((config: ControlPlaneConfig) => config.scaling)
  val module: TaskLayer[ScalingConfig] = ZLayer.make[ScalingConfig](ControlPlaneConfig.module, layer)
}
