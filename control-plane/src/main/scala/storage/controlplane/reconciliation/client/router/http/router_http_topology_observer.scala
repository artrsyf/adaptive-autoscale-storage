package storage.controlplane.reconciliation.client.router.http

import storage.controlplane.reconciliation.domain.ActuationConfig
import storage.controlplane.reconciliation.domain.RouterTopologyError.{InvalidData, Protocol, Unavailable}
import storage.controlplane.reconciliation.domain.{RouterTopologyError, RouterTopologyState}
import storage.controlplane.reconciliation.service.RouterTopologyObserver
import zio.http.{Client, Request, URL}
import zio.json._
import zio.{Duration as ZioDuration, IO, TaskLayer, URLayer, ZIO, ZLayer}

final class RouterHttpTopologyObserver(config: ActuationConfig, client: Client) extends RouterTopologyObserver {
  override def current: IO[RouterTopologyError, RouterTopologyState] = {
    val separator = if (config.routerUrl.endsWith("/")) "" else "/"
    val address = s"${config.routerUrl}${separator}topology"
    for {
      url <- ZIO.fromEither(URL.decode(address)).mapError(error => InvalidData(error.getMessage))
      response <- client
        .batched(Request.get(url))
        .mapError(Unavailable.apply)
        .timeoutFail(Unavailable(new RuntimeException("Router topology request timed out")))(
          ZioDuration.fromMillis(config.requestTimeout.toMillis)
        )
      body <- response.body.asString.mapError(Unavailable.apply)
      _ <- ZIO
        .fail(Protocol(s"Router returned HTTP ${response.status.code}"))
        .unless(response.status.isSuccess)
      topology <- decode(body)
    } yield topology
  }

  private[http] def decode(body: String): IO[RouterTopologyError, RouterTopologyState] =
    for {
      decoded <- ZIO.fromEither(body.fromJson[RouterTopologyResponse]).mapError(InvalidData.apply)
      topology = RouterTopologyState(decoded.epoch, decoded.partitionCount, decoded.nodeIds.toSet)
      _ <- ZIO
        .fail(InvalidData("Router topology contains duplicate node identifiers"))
        .unless(decoded.nodeIds.distinct.size == decoded.nodeIds.size)
      valid <- ZIO.fromEither(topology.validate).mapError(InvalidData.apply)
    } yield valid
}

object RouterHttpTopologyObserver {
  val layer: URLayer[ActuationConfig & Client, RouterTopologyObserver] = ZLayer.derive[RouterHttpTopologyObserver]
  val module: TaskLayer[RouterTopologyObserver] =
    ZLayer.make[RouterTopologyObserver](ActuationConfig.module, Client.default, layer)
}
