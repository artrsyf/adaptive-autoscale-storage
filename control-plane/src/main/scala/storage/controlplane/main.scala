package storage.controlplane

import sttp.tapir.server.ziohttp.ZioHttpInterpreter
import storage.controlplane.app.OperationalRoutes
import storage.controlplane.assignment.repository.postgres.{
  AssignmentPostgresRepository,
  PostgresDataSource,
  PostgresPassword
}
import storage.controlplane.assignment.service.AssignmentService
import storage.controlplane.assignment.transport.http.{AssignmentHttpHandler, AssignmentHttpRoutes}
import storage.controlplane.config.{ApiConfig, ControlPlaneConfig, MetricsConfig, PostgresConfig, ServerConfig}
import storage.controlplane.scaling.domain.ScalingConfig
import storage.controlplane.scaling.service.ControllerService
import storage.controlplane.scaling.transport.http.{ScalingDecisionHttpHandler, ScalingDecisionHttpRoutes}
import storage.controlplane.systemstate.client.prometheus.PrometheusMetricsSource
import storage.controlplane.systemstate.service.{AssignmentServiceSnapshotSource, StateAggregator}
import storage.controlplane.systemstate.transport.http.{SystemStateHttpHandler, SystemStateHttpRoutes}
import zio.http.netty.NettyConfig
import zio.http.{Client, Server}
import zio.{Scope, ZIO, ZIOAppArgs, ZIOAppDefault, ZLayer}

object Main extends ZIOAppDefault {
  override def run: ZIO[ZIOAppArgs & Scope, Any, Any] =
    for {
      arguments <- getArgs
      configPath <- ZIO.fromEither(parseConfigPath(arguments.toList)).mapError(new IllegalArgumentException(_))
      config <- ControlPlaneConfig.load(configPath)
      serverConfig = Server.Config.default
        .binding(config.server.listenHost, config.server.port)
        .idleTimeout(java.time.Duration.ofMillis(config.server.idleTimeout.toMillis))
        .gracefulShutdownTimeout(java.time.Duration.ofMillis(config.server.shutdownTimeout.toMillis))
      _ <- application.provideSome[Scope](
        ZLayer.succeed(config),
        ServerConfig.layer,
        ApiConfig.layer,
        PostgresConfig.layer,
        MetricsConfig.layer,
        ScalingConfig.layer,
        PostgresPassword.module,
        PostgresDataSource.layer,
        AssignmentPostgresRepository.layer,
        AssignmentService.layer,
        AssignmentServiceSnapshotSource.layer,
        Client.default,
        PrometheusMetricsSource.layer,
        StateAggregator.layer,
        ControllerService.layer,
        AssignmentHttpHandler.layer,
        AssignmentHttpRoutes.layer,
        SystemStateHttpHandler.layer,
        SystemStateHttpRoutes.layer,
        ScalingDecisionHttpHandler.layer,
        ScalingDecisionHttpRoutes.layer,
        OperationalRoutes.layer,
        ZLayer.succeed(serverConfig),
        ZLayer.succeed(NettyConfig.default),
        Server.customized
      )
    } yield ()

  private val application =
    for {
      assignmentService <- ZIO.service[AssignmentService]
      controller <- ZIO.service[ControllerService]
      assignmentRoutes <- ZIO.service[AssignmentHttpRoutes]
      systemStateRoutes <- ZIO.service[SystemStateHttpRoutes]
      scalingDecisionRoutes <- ZIO.service[ScalingDecisionHttpRoutes]
      operationalRoutes <- ZIO.service[OperationalRoutes]
      _ <- assignmentService.current
      _ <- controller.run.forkScoped
      domainRoutes = ZioHttpInterpreter().toHttp(
        assignmentRoutes.endpoints.toList ++
          systemStateRoutes.endpoints.toList ++
          scalingDecisionRoutes.endpoints.toList
      )
      _ <- Server.serve(domainRoutes ++ operationalRoutes.routes)
    } yield ()

  private def parseConfigPath(arguments: List[String]): Either[String, String] = arguments match {
    case Nil                       => Right("config.yaml")
    case "--config" :: path :: Nil => Right(path)
    case _                         => Left("usage: control-plane [--config path]")
  }
}
