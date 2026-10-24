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
import storage.controlplane.systemstate.client.prometheus.{
  DockerPrometheusQuerySet,
  KubernetesPrometheusQuerySet,
  PrometheusMetricsProfile,
  PrometheusMetricsSource
}
import storage.controlplane.systemstate.service.{AssignmentServiceSnapshotSource, StateAggregator}
import storage.controlplane.systemstate.transport.http.{SystemStateHttpHandler, SystemStateHttpRoutes}
import zio.http.netty.NettyConfig
import zio.http.{Client, Server}
import zio.{Scope, ZIO, ZIOAppArgs, ZIOAppDefault, ZLayer}

object Main extends ZIOAppDefault {
  override def run: ZIO[ZIOAppArgs & Scope, Any, Any] =
    for {
      arguments <- getArgs
      applicationArguments <- ZIO
        .fromEither(parseArguments(arguments.toList))
        .mapError(new IllegalArgumentException(_))
      config <- ControlPlaneConfig.load(applicationArguments.configPath)
      prometheusQuerySetLayer = applicationArguments.metricsProfile match {
        case PrometheusMetricsProfile.Docker     => DockerPrometheusQuerySet.layer
        case PrometheusMetricsProfile.Kubernetes => KubernetesPrometheusQuerySet.layer
      }
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
        prometheusQuerySetLayer,
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

  private final case class ApplicationArguments(
      configPath: String,
      metricsProfile: PrometheusMetricsProfile
  )

  private def parseArguments(arguments: List[String]): Either[String, ApplicationArguments] = {
    def loop(
        remaining: List[String],
        configPath: Option[String],
        metricsProfile: Option[PrometheusMetricsProfile]
    ): Either[String, ApplicationArguments] = remaining match {
      case Nil =>
        Right(
          ApplicationArguments(
            configPath.getOrElse("config.yaml"),
            metricsProfile.getOrElse(PrometheusMetricsProfile.Docker)
          )
        )
      case "--config" :: path :: tail if configPath.isEmpty =>
        loop(tail, Some(path), metricsProfile)
      case "--metrics-profile" :: value :: tail if metricsProfile.isEmpty =>
        PrometheusMetricsProfile.parse(value).flatMap(profile => loop(tail, configPath, Some(profile)))
      case _ =>
        Left("usage: control-plane [--config path] [--metrics-profile docker|kubernetes]")
    }

    loop(arguments, None, None)
  }
}
