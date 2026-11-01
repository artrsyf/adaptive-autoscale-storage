package storage.controlplane.config

import storage.controlplane.reconciliation.domain.ActuationMode
import zio.test._

object ControlPlaneConfigSpec extends ZIOSpecDefault {
  override def spec = suite("ControlPlaneConfig")(
    test("loads and validates the repository YAML") {
      ControlPlaneConfig.load("config.yaml").map { config =>
        assertTrue(
          config.server.port == 8080,
          config.api.maxBodyBytes == 1048576,
          config.postgres.poolMax == 4,
          config.metrics.pollInterval.toSeconds == 5,
          config.scaling.confirmationWindows == 3,
          config.scaling.thresholds.size == 9,
          config.actuation.mode == ActuationMode.Advisory,
          config.actuation.workloadName == "processing-unit",
          config.validate.isRight
        )
      }
    }
  )
}
