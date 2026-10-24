package storage.controlplane.systemstate.client.prometheus

import java.time.Instant
import java.time.Duration

import storage.controlplane.config.{MetricsConfig, MetricsRetryConfig}
import storage.controlplane.systemstate.domain.SignalName
import storage.controlplane.systemstate.service.MetricsSourceError
import zio.http.Client
import zio.test._

import scala.concurrent.duration._

object PrometheusMetricsSourceSpec extends ZIOSpecDefault {
  private val source = new PrometheusMetricsSource(
    MetricsConfig(
      "http://prometheus:9090",
      5.seconds,
      2.seconds,
      30.seconds,
      15.seconds,
      MetricsRetryConfig(2, 250.millis, 1.second)
    ),
    null.asInstanceOf[Client],
    DockerPrometheusQuerySet
  )

  override def spec = suite("PrometheusMetricsSource")(
    test("decodes a vector and its sample timestamp") {
      val body =
        """{"status":"success","data":{"resultType":"vector","result":[{"metric":{"node":"processing-unit-1"},"value":[1791720000.5,"12.25"]}]}}"""
      source.decode(SignalName.RequestsPerSecond, body).map { samples =>
        assertTrue(
          samples.size == 1,
          samples.head.nodeId == "processing-unit-1",
          samples.head.value == 12.25,
          samples.head.sampledAt == Instant.ofEpochMilli(1791720000500L)
        )
      }
    },
    test("accepts an empty vector without inventing zero samples") {
      val body = """{"status":"success","data":{"resultType":"vector","result":[]}}"""
      source.decode(SignalName.ActiveRequests, body).map(samples => assertTrue(samples.isEmpty))
    },
    test("classifies malformed and error responses") {
      val malformed = source.decode(SignalName.ActiveRequests, "{}").either
      val failed = source
        .decode(
          SignalName.ActiveRequests,
          """{"status":"error","errorType":"bad_data","error":"invalid query"}"""
        )
        .either
      malformed.zip(failed).map { case (malformedResult, failedResult) =>
        assertTrue(
          malformedResult.isLeft,
          failedResult == Left(MetricsSourceError.Protocol("invalid query"))
        )
      }
    },
    test("Docker queries define every signal and preserve the established resource metric contract") {
      val queries = DockerPrometheusQuerySet.all(Duration.ofSeconds(30))
      assertTrue(
        queries.keySet == SignalName.values.toSet,
        queries(SignalName.ServerErrorRate).contains("or on(node)"),
        queries(SignalName.OverloadRate).contains("or on(node)"),
        queries(SignalName.CpuUtilization).contains("container_label_com_docker_compose_service"),
        queries(SignalName.MemoryUtilization).contains("container_label_com_docker_compose_service")
      )
    },
    test("Kubernetes queries implement the same signal contract with pod metrics") {
      val queries = KubernetesPrometheusQuerySet.all(Duration.ofSeconds(30))
      assertTrue(
        queries.keySet == SignalName.values.toSet,
        queries(SignalName.RequestsPerSecond) ==
          DockerPrometheusQuerySet.all(Duration.ofSeconds(30))(SignalName.RequestsPerSecond),
        queries(SignalName.CpuUtilization).contains("kube_pod_container_resource_limits"),
        queries(SignalName.MemoryUtilization).contains("container_memory_working_set_bytes"),
        queries(SignalName.CpuUtilization).contains("\"node\"")
      )
    }
  )
}
