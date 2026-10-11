package storage.controlplane.systemstate.transport.dto

import storage.controlplane.scaling.domain.ControllerSnapshot
import zio.json._
import zio.test._

object SystemStateResponseSpec extends ZIOSpecDefault {
  override def spec = suite("ControlStateResponse")(
    test("represents collecting without fabricated state") {
      val encoded = ControlStateResponse.fromDomain(ControllerSnapshot.collecting).toJson
      assertTrue(encoded == """{"status":"collecting"}""")
    },
    test("uses the snake case wire contract for nested state") {
      val state = SystemStateDto(
        observedAt = "2026-10-11T12:00:00Z",
        windowStartedAt = "2026-10-11T11:59:00Z",
        windowEndedAt = "2026-10-11T12:00:00Z",
        assignmentEpoch = 7,
        expectedNodes = List("processing-unit-1"),
        processingUnits = List(ProcessingUnitStateDto("processing-unit-1", Nil)),
        clusterSignals = Nil,
        totalRps = Some(42),
        dataQuality = DataQualityDto(true, "available", Nil, Nil, Nil, Nil)
      )

      val encoded = ControlStateResponse("healthy", Some(state)).toJson

      assertTrue(
        encoded.contains("\"observed_at\""),
        encoded.contains("\"window_started_at\""),
        encoded.contains("\"assignment_epoch\""),
        encoded.contains("\"processing_units\""),
        encoded.contains("\"node_id\""),
        encoded.contains("\"total_rps\""),
        encoded.contains("\"data_quality\""),
        !encoded.contains("observedAt"),
        !encoded.contains("nodeId")
      )
    }
  )
}
