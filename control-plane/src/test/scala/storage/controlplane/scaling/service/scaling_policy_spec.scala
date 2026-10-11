package storage.controlplane.scaling.service

import java.time.{Duration, Instant}

import storage.controlplane.scaling.domain._
import storage.controlplane.systemstate.domain._
import zio.test._

object ScalingPolicySpec extends ZIOSpecDefault {
  private val evaluatedAt = Instant.parse("2026-10-11T12:00:00Z")
  private val thresholds = SignalName.policySignals.map(signal => signal -> ThresholdBand(0.25, 0.75)).toMap
  private val config = ScalingConfig(1, 5, 1, 2, Duration.ofSeconds(30), thresholds)

  override def spec = suite("ScalingPolicy")(
    test("requires complete fresh data") {
      val incomplete = state(0.5).copy(dataQuality =
        quality(complete = false).copy(
          missingSignals = Set(SignalReference(SignalName.CpuUtilization, Some("one")))
        )
      )
      val (decision, next) = ScalingPolicy.evaluate(incomplete, PolicyState.empty, config, evaluatedAt)
      assertTrue(
        decision.kind == ScalingDecisionKind.InsufficientData,
        decision.blockedBy == List("missing_signals"),
        next.confirmationCount == 0
      )
    },
    test("explains unexpected topology series") {
      val unexpected = state(0.5).copy(dataQuality = quality(complete = false).copy(unknownNodes = Set("other")))
      val (decision, _) = ScalingPolicy.evaluate(unexpected, PolicyState.empty, config, evaluatedAt)
      assertTrue(
        decision.kind == ScalingDecisionKind.InsufficientData,
        decision.blockedBy == List("unknown_nodes")
      )
    },
    test("confirms scale out over consecutive windows") {
      val overloaded = state(0.5, Map(SignalName.CpuUtilization -> 0.9))
      val (first, pending) = ScalingPolicy.evaluate(overloaded, PolicyState.empty, config, evaluatedAt)
      val (second, completed) = ScalingPolicy.evaluate(overloaded, pending, config, evaluatedAt.plusSeconds(5))
      assertTrue(
        first.kind == ScalingDecisionKind.Maintain,
        first.blockedBy == List("confirmation:1/2"),
        second.kind == ScalingDecisionKind.ScaleOut,
        second.recommendedReplicas == 3,
        completed.lastRecommendationAt.contains(evaluatedAt.plusSeconds(5))
      )
    },
    test("treats every configured policy signal as an independent scale-out trigger") {
      val decisions = SignalName.policySignals.toList.map { signal =>
        val overloaded = state(0.5, Map(signal -> 0.9))
        val (_, pending) = ScalingPolicy.evaluate(overloaded, PolicyState.empty, config, evaluatedAt)
        val (scaleOut, _) = ScalingPolicy.evaluate(overloaded, pending, config, evaluatedAt.plusSeconds(5))
        signal -> scaleOut
      }
      assertTrue(decisions.forall { case (signal, scaleOut) =>
        scaleOut.kind == ScalingDecisionKind.ScaleOut &&
        scaleOut.reasons.exists(reason => reason.signal == signal && reason.value == 0.9 && reason.threshold == 0.75)
      })
    },
    test("scales in only when every signal is below its lower threshold") {
      val low = state(0.1)
      val oneHigh = state(0.1, Map(SignalName.HttpLatencyP99 -> 0.5))
      val (_, pending) = ScalingPolicy.evaluate(low, PolicyState.empty, config, evaluatedAt)
      val (scaleIn, _) = ScalingPolicy.evaluate(low, pending, config, evaluatedAt.plusSeconds(5))
      val (maintain, _) = ScalingPolicy.evaluate(oneHigh, PolicyState.empty, config, evaluatedAt)
      assertTrue(
        scaleIn.kind == ScalingDecisionKind.ScaleIn,
        scaleIn.recommendedReplicas == 1,
        maintain.kind == ScalingDecisionKind.Maintain
      )
    },
    test("enforces cooldown and replica bounds") {
      val overloaded = state(0.9)
      val cooldownState = PolicyState(Some(ScalingDirection.Out), 1, Some(evaluatedAt.minusSeconds(10)))
      val (cooldownDecision, _) = ScalingPolicy.evaluate(overloaded, cooldownState, config, evaluatedAt)
      val maxState = state(0.9, nodes = Set("one", "two", "three", "four", "five"))
      val (maxDecision, _) =
        ScalingPolicy.evaluate(maxState, PolicyState(Some(ScalingDirection.Out), 1, None), config, evaluatedAt)
      assertTrue(
        cooldownDecision.blockedBy == List("cooldown:20s"),
        maxDecision.kind == ScalingDecisionKind.Maintain,
        maxDecision.blockedBy == List("max_replicas")
      )
    },
    test("is deterministic for identical state and policy history") {
      val input = state(0.9)
      val policyState = PolicyState(Some(ScalingDirection.Out), 1, None)
      val first = ScalingPolicy.evaluate(input, policyState, config, evaluatedAt)
      val second = ScalingPolicy.evaluate(input, policyState, config, evaluatedAt)
      assertTrue(first == second)
    }
  )

  private def state(
      defaultValue: Double,
      overrides: Map[SignalName, Double] = Map.empty,
      nodes: Set[String] = Set("one", "two")
  ): SystemState = {
    val values = SignalName.policySignals.map { signal =>
      val value = overrides.getOrElse(signal, defaultValue)
      signal -> MetricValue.make(value, signal.unit, evaluatedAt).toOption.get
    }.toMap
    val observed = values.map { case (signal, value) =>
      signal -> ObservedMetric(Some(value), DataFreshness(FreshnessKind.Fresh, Some(Duration.ZERO)))
    }
    val units = nodes.toList.sorted.map(node => ProcessingUnitState(node, observed))
    SystemState(
      evaluatedAt,
      MetricWindow(evaluatedAt.minusSeconds(30), evaluatedAt),
      7,
      nodes,
      units,
      Some(DatabaseState(values(SignalName.PoolAcquireP95), values(SignalName.DatabaseOperationP95))),
      ClusterState(values, Some(values(SignalName.RequestsPerSecond))),
      quality(complete = true)
    )
  }

  private def quality(complete: Boolean): DataQuality =
    DataQuality(complete, MetricsSourceStatus.Available, Set.empty, Set.empty, Set.empty, Set.empty)
}
