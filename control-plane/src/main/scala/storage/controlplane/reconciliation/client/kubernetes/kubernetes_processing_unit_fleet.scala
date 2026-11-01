package storage.controlplane.reconciliation.client.kubernetes

import io.kubernetes.client.openapi.{ApiClient, ApiException, Configuration}
import io.kubernetes.client.openapi.apis.{AppsV1Api, CoreV1Api}
import io.kubernetes.client.openapi.models.{V1Pod, V1ScaleSpec}
import io.kubernetes.client.util.Config
import storage.controlplane.assignment.domain.Node
import storage.controlplane.reconciliation.domain.FleetError.{Conflict, Forbidden, InvalidState, Unavailable}
import storage.controlplane.reconciliation.domain.{ActuationConfig, FleetError, FleetSnapshot, ReplicaCount}
import storage.controlplane.reconciliation.service.ProcessingUnitFleet
import zio.{IO, TaskLayer, URLayer, ZIO, ZLayer}

import scala.jdk.CollectionConverters._

final class KubernetesProcessingUnitFleet(
    config: ActuationConfig,
    appsApi: AppsV1Api,
    coreApi: CoreV1Api
) extends ProcessingUnitFleet {
  override def observe: IO[FleetError, FleetSnapshot] =
    mapApiErrors(
      ZIO
        .attemptBlocking {
          val statefulSet = appsApi.readNamespacedStatefulSet(config.workloadName, config.namespace).execute()
          val pods = coreApi
            .listNamespacedPod(config.namespace)
            .labelSelector(s"app.kubernetes.io/name=${config.workloadName}")
            .execute()
            .getItems
            .asScala
            .toList
          val desired = Option(statefulSet.getSpec).flatMap(spec => Option(spec.getReplicas)).map(_.intValue())
          val observed =
            Option(statefulSet.getStatus).flatMap(status => Option(status.getCurrentReplicas)).map(_.intValue())
          val readyMembers = pods.filter(isReady).map(toNode).toSet
          for {
            desiredReplicas <- desired.toRight("StatefulSet has no desired replica count")
            observedReplicas <- observed.toRight("StatefulSet has no observed replica count")
            snapshot = FleetSnapshot(ReplicaCount(desiredReplicas), ReplicaCount(observedReplicas), readyMembers)
            valid <- snapshot.validate
          } yield valid
        }
        .flatMap(result => ZIO.fromEither(result).mapError(InvalidState.apply))
    )

  override def retainedMembersAfterResize(target: ReplicaCount): IO[FleetError, Set[Node]] =
    observe.flatMap { snapshot =>
      val ordered = snapshot.readyMembers.toList.sortBy(node => ordinal(node.id))
      if (target.value < 1 || target.value >= ordered.size)
        ZIO.fail(InvalidState("scale-in target must retain a non-empty strict subset of ready members"))
      else ZIO.succeed(ordered.take(target.value).toSet)
    }

  override def resize(target: ReplicaCount): IO[FleetError, Unit] =
    if (target.value < 1) ZIO.fail(InvalidState("replica target must be positive"))
    else
      mapApiErrors(
        ZIO.attemptBlocking {
          val scale = appsApi.readNamespacedStatefulSetScale(config.workloadName, config.namespace).execute()
          val updated = Option(scale.getSpec).fold(new V1ScaleSpec())(identity)
          updated.setReplicas(target.value)
          scale.setSpec(updated)
          appsApi.replaceNamespacedStatefulSetScale(config.workloadName, config.namespace, scale).execute()
          ()
        }
      )

  private def isReady(pod: V1Pod): Boolean = {
    val running = Option(pod.getStatus).flatMap(status => Option(status.getPhase)).contains("Running")
    val ready = Option(pod.getStatus)
      .flatMap(status => Option(status.getConditions))
      .toList
      .flatMap(_.asScala)
      .exists(condition => condition.getType == "Ready" && condition.getStatus == "True")
    val terminating = Option(pod.getMetadata).flatMap(metadata => Option(metadata.getDeletionTimestamp)).isDefined
    running && ready && !terminating
  }

  private def toNode(pod: V1Pod): Node = {
    val nodeId = Option(pod.getMetadata).flatMap(metadata => Option(metadata.getName)).getOrElse("")
    Node(nodeId, s"http://$nodeId.${config.headlessService}:${config.applicationPort}")
  }

  private def ordinal(nodeId: String): Int =
    nodeId.split("-").lastOption.flatMap(_.toIntOption).getOrElse(Int.MaxValue)

  private def mapApiErrors[A](effect: IO[Throwable, A]): IO[FleetError, A] =
    effect.mapError {
      case apiError: ApiException if apiError.getCode == 403 => Forbidden(apiError.getResponseBody)
      case apiError: ApiException if apiError.getCode == 409 => Conflict(apiError.getResponseBody)
      case error                                             => Unavailable(error)
    }
}

object KubernetesProcessingUnitFleet {
  val apiClientLayer: TaskLayer[ApiClient] = ZLayer.fromZIO(
    ZIO.attempt {
      val client = Config.fromCluster()
      Configuration.setDefaultApiClient(client)
      client
    }
  )
  val appsApiLayer: URLayer[ApiClient, AppsV1Api] = ZLayer.fromFunction((client: ApiClient) => new AppsV1Api(client))
  val coreApiLayer: URLayer[ApiClient, CoreV1Api] = ZLayer.fromFunction((client: ApiClient) => new CoreV1Api(client))
  val layer: URLayer[ActuationConfig & AppsV1Api & CoreV1Api, ProcessingUnitFleet] =
    ZLayer.derive[KubernetesProcessingUnitFleet]
  val module: TaskLayer[ProcessingUnitFleet] = ZLayer.make[ProcessingUnitFleet](
    ActuationConfig.module,
    apiClientLayer,
    appsApiLayer,
    coreApiLayer,
    layer
  )
  val runtimeLayer: ZLayer[ActuationConfig, Throwable, ProcessingUnitFleet] =
    ZLayer.makeSome[ActuationConfig, ProcessingUnitFleet](
      apiClientLayer,
      appsApiLayer,
      coreApiLayer,
      layer
    )
}
