package storage.controlplane.reconciliation.client.router.http

import storage.controlplane.reconciliation.domain.{ActuationConfig, ActuationMode}
import zio.http.Client
import zio.test._
import zio.ZIO

import scala.concurrent.duration._

object RouterHttpTopologyObserverSpec extends ZIOSpecDefault {
  private val config = ActuationConfig(
    ActuationMode.Advisory,
    2.seconds,
    1.second,
    2.minutes,
    "http://router:8080",
    "autoscale-storage",
    "processing-unit",
    "processing-unit-headless",
    8080
  )

  override def spec = suite("RouterHttpTopologyObserver")(
    test("decodes a complete topology") {
      for {
        client <- ZIO.service[Client]
        observer = RouterHttpTopologyObserver(config, client)
        topology <- observer.decode(
          """{"epoch":7,"partition_count":128,"node_ids":["processing-unit-1","processing-unit-2"]}"""
        )
      } yield assertTrue(
        topology.epoch == 7,
        topology.partitionCount == 128,
        topology.nodeIds == Set("processing-unit-1", "processing-unit-2")
      )
    },
    test("rejects duplicate nodes and unknown fields") {
      for {
        client <- ZIO.service[Client]
        observer = RouterHttpTopologyObserver(config, client)
        duplicate <- observer
          .decode("""{"epoch":7,"partition_count":128,"node_ids":["one","one"]}""")
          .either
        unknown <- observer
          .decode("""{"epoch":7,"partition_count":128,"node_ids":["one"],"pods":1}""")
          .either
      } yield assertTrue(duplicate.isLeft, unknown.isLeft)
    }
  ).provideLayer(Client.default)
}
