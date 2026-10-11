package storage.controlplane.assignment.repository.postgres

import storage.controlplane.assignment.domain.AssignmentError.EpochConflict
import storage.controlplane.config.ControlPlaneConfig
import zio.test._
import zio.{System => ZioSystem, ZIO}

object AssignmentPostgresRepositoryIntegrationSpec extends ZIOSpecDefault {
  override def spec = suite("AssignmentPostgresRepository integration")(
    test("reads the active snapshot and rejects a stale CAS") {
      ZioSystem.env("POSTGRES_PASSWORD").flatMap {
        case None => ZIO.succeed(assertTrue(true))
        case Some(password) =>
          ZIO.scoped {
            for {
              config <- ControlPlaneConfig.load("config.yaml")
              transactor <- PostgresDataSource.make(config.postgres, password)
              repository = new AssignmentPostgresRepository(transactor)
              snapshot <- repository.current
              conflict <- repository.publish(snapshot.epoch + 1, snapshot.copy(epoch = snapshot.epoch + 1)).either
            } yield assertTrue(snapshot.validate.isRight, conflict == Left(EpochConflict))
          }
      }
    }
  )
}
