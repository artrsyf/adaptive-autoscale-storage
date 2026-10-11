package storage.controlplane

import sttp.tapir.server.ziohttp.ZioHttpInterpreter
import storage.controlplane.app.OperationalRoutes
import storage.controlplane.assignment.repository.postgres.{AssignmentPostgresRepository, PostgresDataSource}
import storage.controlplane.assignment.service.AssignmentService
import storage.controlplane.assignment.transport.http.{AssignmentHttpHandler, AssignmentHttpRoutes}
import storage.controlplane.config.ControlPlaneConfig
import zio.http.netty.NettyConfig
import zio.http.Server
import zio.{Scope, ZIO, ZIOAppArgs, ZIOAppDefault, ZLayer}

object Main extends ZIOAppDefault {
  override def run: ZIO[ZIOAppArgs with Scope, Any, Any] =
    for {
      arguments <- getArgs
      configPath <- ZIO.fromEither(parseConfigPath(arguments.toList)).mapError(new IllegalArgumentException(_))
      config <- ControlPlaneConfig.load(configPath)
      password <- ZIO
        .fromOption(sys.env.get("POSTGRES_PASSWORD"))
        .orElseFail(new IllegalArgumentException("POSTGRES_PASSWORD is required"))
      transactor <- PostgresDataSource.make(config.postgres, password)
      repository = new AssignmentPostgresRepository(transactor)
      service <- AssignmentService.make(repository)
      _ <- service.current
      assignmentHandler = new AssignmentHttpHandler(service, config.api)
      assignmentRoutes = ZioHttpInterpreter().toHttp(
        new AssignmentHttpRoutes(assignmentHandler, config.api).endpoints.toList
      )
      operationalRoutes = OperationalRoutes.make(repository, service, config.server)
      routes = assignmentRoutes ++ operationalRoutes
      serverConfig = Server.Config.default
        .binding(config.server.listenHost, config.server.port)
        .idleTimeout(java.time.Duration.ofMillis(config.server.idleTimeout.toMillis))
        .gracefulShutdownTimeout(java.time.Duration.ofMillis(config.server.shutdownTimeout.toMillis))
      _ <- Server
        .serve(routes)
        .provide(
          ZLayer.succeed(serverConfig),
          ZLayer.succeed(NettyConfig.default),
          Server.customized
        )
    } yield ()

  private def parseConfigPath(arguments: List[String]): Either[String, String] = arguments match {
    case Nil                       => Right("config.yaml")
    case "--config" :: path :: Nil => Right(path)
    case _                         => Left("usage: control-plane [--config path]")
  }
}
