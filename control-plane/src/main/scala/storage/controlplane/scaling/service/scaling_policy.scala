package storage.controlplane.scaling.service

import java.time.{Duration, Instant}

import storage.controlplane.scaling.domain._
import storage.controlplane.systemstate.domain.{SignalName, SystemState}

object ScalingPolicy {
  def evaluate(
      systemState: SystemState,
      policyState: PolicyState,
      config: ScalingConfig,
      evaluatedAt: Instant
  ): (ScalingDecision, PolicyState) = {
    val currentReplicas = systemState.expectedNodes.size
    if (!systemState.dataQuality.complete || !SignalName.policySignals.subsetOf(systemState.cluster.signals.keySet)) {
      val blocked = dataQualityBlocks(systemState)
      decision(
        ScalingDecisionKind.InsufficientData,
        currentReplicas,
        currentReplicas,
        evaluatedAt,
        systemState,
        Nil,
        blocked
      ) ->
        policyState.copy(pendingDirection = None, confirmationCount = 0)
    } else {
      val upperBreaches = SignalName.policySignals.toList.flatMap { signal =>
        val metric = systemState.cluster.signals(signal)
        val threshold = config.thresholds(signal).upper
        Option.when(metric.value >= threshold)(ScalingReason(signal, metric.value, metric.unit, ">=", threshold))
      }
      val lowerSignals = SignalName.policySignals.toList.map { signal =>
        val metric = systemState.cluster.signals(signal)
        val threshold = config.thresholds(signal).lower
        ScalingReason(signal, metric.value, metric.unit, "<=", threshold)
      }
      val direction =
        if (upperBreaches.nonEmpty) Some(ScalingDirection.Out)
        else if (lowerSignals.forall(reason => reason.value <= reason.threshold)) Some(ScalingDirection.In)
        else None

      direction match {
        case None =>
          decision(
            ScalingDecisionKind.Maintain,
            currentReplicas,
            currentReplicas,
            evaluatedAt,
            systemState,
            Nil,
            Nil
          ) ->
            policyState.copy(pendingDirection = None, confirmationCount = 0)
        case Some(candidate) =>
          evaluateCandidate(
            candidate,
            if (candidate == ScalingDirection.Out) upperBreaches else lowerSignals,
            systemState,
            policyState,
            config,
            evaluatedAt
          )
      }
    }
  }

  private def evaluateCandidate(
      direction: ScalingDirection,
      reasons: List[ScalingReason],
      systemState: SystemState,
      policyState: PolicyState,
      config: ScalingConfig,
      evaluatedAt: Instant
  ): (ScalingDecision, PolicyState) = {
    val currentReplicas = systemState.expectedNodes.size
    val confirmationCount =
      if (policyState.pendingDirection.contains(direction)) policyState.confirmationCount + 1 else 1
    val pendingState = policyState.copy(pendingDirection = Some(direction), confirmationCount = confirmationCount)
    val confirmationBlock = Option.when(confirmationCount < config.confirmationWindows)(
      s"confirmation:$confirmationCount/${config.confirmationWindows}"
    )
    val cooldownBlock = policyState.lastRecommendationAt.flatMap { lastRecommendationAt =>
      val elapsed = Duration.between(lastRecommendationAt, evaluatedAt)
      Option.when(elapsed.compareTo(config.cooldown) < 0)(s"cooldown:${config.cooldown.minus(elapsed).toSeconds}s")
    }
    val boundBlock = direction match {
      case ScalingDirection.Out => Option.when(currentReplicas >= config.maxReplicas)("max_replicas")
      case ScalingDirection.In  => Option.when(currentReplicas <= config.minReplicas)("min_replicas")
    }
    val blocks = List(confirmationBlock, cooldownBlock, boundBlock).flatten
    if (blocks.nonEmpty)
      decision(
        ScalingDecisionKind.Maintain,
        currentReplicas,
        currentReplicas,
        evaluatedAt,
        systemState,
        reasons,
        blocks
      ) -> pendingState
    else {
      val (kind, recommendedReplicas) = direction match {
        case ScalingDirection.Out =>
          ScalingDecisionKind.ScaleOut -> math.min(config.maxReplicas, currentReplicas + config.step)
        case ScalingDirection.In =>
          ScalingDecisionKind.ScaleIn -> math.max(config.minReplicas, currentReplicas - config.step)
      }
      decision(kind, currentReplicas, recommendedReplicas, evaluatedAt, systemState, reasons, Nil) ->
        PolicyState(None, 0, Some(evaluatedAt))
    }
  }

  private def dataQualityBlocks(systemState: SystemState): List[String] = {
    val missing = Option.when(systemState.dataQuality.missingSignals.nonEmpty)("missing_signals")
    val stale = Option.when(systemState.dataQuality.staleSignals.nonEmpty)("stale_signals")
    val duplicate = Option.when(systemState.dataQuality.duplicateSignals.nonEmpty)("duplicate_signals")
    val unknown = Option.when(systemState.dataQuality.unknownNodes.nonEmpty)("unknown_nodes")
    val source =
      Option.when(systemState.dataQuality.sourceStatus.wireName == "unavailable")("metrics_source_unavailable")
    List(missing, stale, duplicate, unknown, source).flatten
  }

  private def decision(
      kind: ScalingDecisionKind,
      currentReplicas: Int,
      recommendedReplicas: Int,
      evaluatedAt: Instant,
      systemState: SystemState,
      reasons: List[ScalingReason],
      blockedBy: List[String]
  ): ScalingDecision = ScalingDecision(
    kind,
    currentReplicas,
    recommendedReplicas,
    evaluatedAt,
    systemState.assignmentEpoch,
    reasons.sortBy(_.signal.wireName),
    blockedBy.sorted
  )
}
