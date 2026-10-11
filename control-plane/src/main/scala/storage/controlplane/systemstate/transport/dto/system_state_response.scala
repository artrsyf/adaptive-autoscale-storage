package storage.controlplane.systemstate.transport.dto

import storage.controlplane.scaling.domain.ControllerSnapshot
import storage.controlplane.systemstate.domain._
import sttp.tapir.Schema
import tofu.logging.Loggable
import tofu.logging.derivation.derived
import zio.json.{jsonMemberNames, JsonCodec, SnakeCase}

@jsonMemberNames(SnakeCase)
final case class SignalValueDto(
    signal: String,
    value: Option[Double],
    unit: String,
    freshness: String,
    sampledAt: Option[String],
    ageSeconds: Option[Double]
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object SignalValueDto {
  def fromDomain(signal: SignalName, observed: ObservedMetric): SignalValueDto = SignalValueDto(
    signal.wireName,
    observed.value.map(_.value),
    signal.unit.wireName,
    observed.freshness.kind.wireName,
    observed.value.map(_.sampledAt.toString),
    observed.freshness.age.map(_.toMillis.toDouble / 1000)
  )
}

@jsonMemberNames(SnakeCase)
final case class ProcessingUnitStateDto(nodeId: String, signals: List[SignalValueDto])
    derives JsonCodec,
      Loggable,
      CanEqual,
      Schema

@jsonMemberNames(SnakeCase)
final case class DataQualityDto(
    complete: Boolean,
    sourceStatus: String,
    missingSignals: List[String],
    staleSignals: List[String],
    duplicateSignals: List[String],
    unknownNodes: List[String]
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema

@jsonMemberNames(SnakeCase)
final case class SystemStateDto(
    observedAt: String,
    windowStartedAt: String,
    windowEndedAt: String,
    assignmentEpoch: Long,
    expectedNodes: List[String],
    processingUnits: List[ProcessingUnitStateDto],
    clusterSignals: List[SignalValueDto],
    totalRps: Option[Double],
    dataQuality: DataQualityDto
) derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object SystemStateDto {
  def fromDomain(state: SystemState): SystemStateDto = {
    def references(values: Set[SignalReference]): List[String] =
      values.toList.map(reference => s"${reference.nodeId.getOrElse("cluster")}:${reference.signal.wireName}").sorted

    val processingUnits = state.processingUnits.map { unit =>
      ProcessingUnitStateDto(
        unit.nodeId,
        SignalName.values.toList.map(signal => SignalValueDto.fromDomain(signal, unit.signals(signal)))
      )
    }
    val clusterSignals = state.cluster.signals.toList.sortBy(_._1.wireName).map { case (signal, metric) =>
      SignalValueDto(
        signal.wireName,
        Some(metric.value),
        metric.unit.wireName,
        FreshnessKind.Fresh.wireName,
        Some(metric.sampledAt.toString),
        Some(java.time.Duration.between(metric.sampledAt, state.observedAt).toMillis.toDouble / 1000)
      )
    }
    SystemStateDto(
      state.observedAt.toString,
      state.window.startedAt.toString,
      state.window.endedAt.toString,
      state.assignmentEpoch,
      state.expectedNodes.toList.sorted,
      processingUnits,
      clusterSignals,
      state.cluster.totalRequestsPerSecond.map(_.value),
      DataQualityDto(
        state.dataQuality.complete,
        state.dataQuality.sourceStatus.wireName,
        references(state.dataQuality.missingSignals),
        references(state.dataQuality.staleSignals),
        references(state.dataQuality.duplicateSignals),
        state.dataQuality.unknownNodes.toList.sorted
      )
    )
  }
}

@jsonMemberNames(SnakeCase)
final case class ControlStateResponse(status: String, state: Option[SystemStateDto])
    derives JsonCodec,
      Loggable,
      CanEqual,
      Schema
object ControlStateResponse {
  def fromDomain(snapshot: ControllerSnapshot): ControlStateResponse =
    ControlStateResponse(snapshot.status.wireName, snapshot.systemState.map(SystemStateDto.fromDomain))
}
