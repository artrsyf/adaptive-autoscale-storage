package storage.controlplane.config

import zio.test._

object ControlPlaneConfigSpec extends ZIOSpecDefault {
  override def spec = suite("ControlPlaneConfig")(
    test("loads and validates the repository YAML") {
      ControlPlaneConfig.load("config.yaml").map { config =>
        assertTrue(
          config.server.port == 8080,
          config.api.maxBodyBytes == 1048576,
          config.postgres.poolMax == 4,
          config.validate.isRight
        )
      }
    }
  )
}
