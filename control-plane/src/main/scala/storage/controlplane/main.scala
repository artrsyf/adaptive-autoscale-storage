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
import storage.controlplane.reconciliation.client.advisory.AdvisoryProcessingUnitFleet
import storage.controlplane.reconciliation.client.kubernetes.KubernetesProcessingUnitFleet
import storage.controlplane.reconciliation.client.router.http.RouterHttpTopologyObserver
import storage.controlplane.reconciliation.domain.{ActuationConfig, ActuationMode}
import storage.controlplane.reconciliation.repository.postgres.ScalingOperationPostgresRepository
import storage.controlplane.reconciliation.service.{
  ActuationController,
  BalancedPartitionAllocator,
  ScalingOperationService,
  TopologyReconciler
}
import storage.controlplane.reconciliation.transport.http.{ReconciliationHttpHandler, ReconciliationHttpRoutes}
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
      loadedConfig <- ControlPlaneConfig.load(applicationArguments.configPath)
      config = applicationArguments.actuationMode.fold(loadedConfig)(mode =>
        loadedConfig.copy(actuation = loadedConfig.actuation.copy(mode = mode))
      )
      prometheusQuerySetLayer = applicationArguments.metricsProfile match {
        case PrometheusMetricsProfile.Docker     => DockerPrometheusQuerySet.layer
        case PrometheusMetricsProfile.Kubernetes => KubernetesPrometheusQuerySet.layer
      }
      processingUnitFleetLayer = config.actuation.mode match {
        case ActuationMode.Advisory   => AdvisoryProcessingUnitFleet.layer
        case ActuationMode.Kubernetes => KubernetesProcessingUnitFleet.runtimeLayer
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
        ActuationConfig.layer,
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
        ScalingOperationPostgresRepository.layer,
        ScalingOperationService.layer,
        BalancedPartitionAllocator.layer,
        RouterHttpTopologyObserver.layer,
        processingUnitFleetLayer,
        TopologyReconciler.layer,
        ActuationController.layer,
        ReconciliationHttpHandler.layer,
        ReconciliationHttpRoutes.layer,
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
      actuationController <- ZIO.service[ActuationController]
      assignmentRoutes <- ZIO.service[AssignmentHttpRoutes]
      systemStateRoutes <- ZIO.service[SystemStateHttpRoutes]
      scalingDecisionRoutes <- ZIO.service[ScalingDecisionHttpRoutes]
      operationalRoutes <- ZIO.service[OperationalRoutes]
      reconciliationRoutes <- ZIO.service[ReconciliationHttpRoutes]
      _ <- assignmentService.current
      _ <- controller.run.forkScoped
      _ <- actuationController.run.forkScoped
      domainRoutes = ZioHttpInterpreter().toHttp(
        assignmentRoutes.endpoints.toList ++
          systemStateRoutes.endpoints.toList ++
          scalingDecisionRoutes.endpoints.toList
          ++ reconciliationRoutes.endpoints.toList
      )
      _ <- Server.serve(domainRoutes ++ operationalRoutes.routes)
    } yield ()

  private final case class ApplicationArguments(
      configPath: String,
      metricsProfile: PrometheusMetricsProfile,
      actuationMode: Option[ActuationMode]
  )

  private def parseArguments(arguments: List[String]): Either[String, ApplicationArguments] = {
    def loop(
        remaining: List[String],
        configPath: Option[String],
        metricsProfile: Option[PrometheusMetricsProfile],
        actuationMode: Option[ActuationMode]
    ): Either[String, ApplicationArguments] = remaining match {
      case Nil =>
        Right(
          ApplicationArguments(
            configPath.getOrElse("config.yaml"),
            metricsProfile.getOrElse(PrometheusMetricsProfile.Docker),
            actuationMode
          )
        )
      case "--config" :: path :: tail if configPath.isEmpty =>
        loop(tail, Some(path), metricsProfile, actuationMode)
      case "--metrics-profile" :: value :: tail if metricsProfile.isEmpty =>
        PrometheusMetricsProfile.parse(value).flatMap(profile => loop(tail, configPath, Some(profile), actuationMode))
      case "--actuation-mode" :: value :: tail if actuationMode.isEmpty =>
        ActuationMode.parse(value).flatMap(mode => loop(tail, configPath, metricsProfile, Some(mode)))
      case _ =>
        Left(
          "usage: control-plane [--config path] [--metrics-profile docker|kubernetes] " +
            "[--actuation-mode advisory|kubernetes]"
        )
    }

    loop(arguments, None, None, None)
  }
}
