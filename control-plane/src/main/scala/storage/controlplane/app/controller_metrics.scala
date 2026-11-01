package storage.controlplane.app

import java.time.{Duration, Instant}

import storage.controlplane.reconciliation.domain.{
  FleetSnapshot,
  ReconciliationPhase,
  ReconciliationStatus,
  ScalingOperation
}
import storage.controlplane.scaling.domain.{ControllerSnapshot, ControllerStatus, ScalingConfig, ScalingDecisionKind}
import storage.controlplane.systemstate.domain.SignalName

object ControllerMetrics {
  def render(
      snapshot: ControllerSnapshot,
      config: ScalingConfig,
      now: Instant,
      assignmentEpoch: Long,
      latestOperation: Option[ScalingOperation] = None,
      fleet: Option[FleetSnapshot] = None
  ): String = {
    val builder = new StringBuilder
    gauge(builder, "storage_assignment_epoch", "Active assignment epoch.", assignmentEpoch.toDouble)
    familyHeader(builder, "storage_control_controller_status", "Current controller status as a one-hot gauge.")
    ControllerStatus.values.foreach(status =>
      sample(
        builder,
        "storage_control_controller_status",
        List("status" -> status.wireName),
        boolean(snapshot.status == status)
      )
    )
    familyHeader(builder, "storage_control_decision", "Current advisory scaling decision as a one-hot gauge.")
    ScalingDecisionKind.values.foreach(kind =>
      sample(
        builder,
        "storage_control_decision",
        List("decision" -> kind.wireName),
        boolean(snapshot.decision.exists(_.kind == kind))
      )
    )
    familyHeader(
      builder,
      "storage_control_cycles_total",
      "Controller cycles by bounded outcome.",
      metricType = "counter"
    )
    sample(builder, "storage_control_cycles_total", List("outcome" -> "complete"), snapshot.completeCycles.toDouble)
    sample(builder, "storage_control_cycles_total", List("outcome" -> "incomplete"), snapshot.incompleteCycles.toDouble)
    sample(builder, "storage_control_cycles_total", List("outcome" -> "failed"), snapshot.failedCycles.toDouble)
    snapshot.lastCycleDuration.foreach(duration =>
      gauge(
        builder,
        "storage_control_cycle_duration_seconds",
        "Duration of the latest completed controller cycle.",
        duration.toNanos.toDouble / 1e9
      )
    )
    snapshot.lastCompleteStateAt.foreach(observedAt =>
      gauge(
        builder,
        "storage_control_last_complete_state_age_seconds",
        "Age of the latest complete system state.",
        math.max(0, Duration.between(observedAt, now).toMillis).toDouble / 1000
      )
    )
    familyHeader(
      builder,
      "storage_control_decision_transitions_total",
      "Decision transitions by decision kind.",
      "counter"
    )
    ScalingDecisionKind.values.foreach(kind =>
      sample(
        builder,
        "storage_control_decision_transitions_total",
        List("decision" -> kind.wireName),
        snapshot.decisionTransitions(kind).toDouble
      )
    )
    gauge(
      builder,
      "storage_control_confirmation_progress_ratio",
      "Current confirmation progress from zero to one.",
      math.min(1.0, snapshot.policyState.confirmationCount.toDouble / config.confirmationWindows)
    )
    val cooldownRemaining = snapshot.policyState.lastRecommendationAt
      .map(last => config.cooldown.minus(Duration.between(last, now)))
      .filterNot(_.isNegative)
      .map(_.toMillis.toDouble / 1000)
      .getOrElse(0.0)
    gauge(
      builder,
      "storage_control_cooldown_remaining_seconds",
      "Remaining advisory recommendation cooldown.",
      cooldownRemaining
    )
    snapshot.decision.foreach { decision =>
      gauge(
        builder,
        "storage_control_current_replicas",
        "Replicas represented by the active assignment.",
        decision.currentReplicas
      )
      gauge(
        builder,
        "storage_control_recommended_replicas",
        "Latest advisory replica recommendation.",
        decision.recommendedReplicas
      )
    }
    renderReconciliation(builder, latestOperation, now)
    renderFleet(builder, snapshot, fleet)
    renderSystemState(builder, snapshot, config, now)
    builder.result()
  }

  private def renderReconciliation(
      builder: StringBuilder,
      latestOperation: Option[ScalingOperation],
      now: Instant
  ): Unit = latestOperation.foreach { operation =>
    familyHeader(
      builder,
      "storage_control_reconciliation_phase",
      "Latest reconciliation phase as a one-hot gauge."
    )
    ReconciliationPhase.values.foreach(phase =>
      sample(
        builder,
        "storage_control_reconciliation_phase",
        List("phase" -> phase.wireName),
        boolean(operation.phase == phase)
      )
    )
    familyHeader(
      builder,
      "storage_control_reconciliation_status",
      "Latest reconciliation status as a one-hot gauge."
    )
    ReconciliationStatus.values.foreach(status =>
      sample(
        builder,
        "storage_control_reconciliation_status",
        List("status" -> status.wireName),
        boolean(operation.status == status)
      )
    )
    familyHeader(builder, "storage_control_reconciliation_direction", "Latest reconciliation direction.")
    List("out", "in").foreach(direction =>
      sample(
        builder,
        "storage_control_reconciliation_direction",
        List("direction" -> direction),
        boolean(operation.direction.toString.equalsIgnoreCase(direction))
      )
    )
    gauge(
      builder,
      "storage_control_reconciliation_source_replicas",
      "Replica count at the start of the latest reconciliation.",
      operation.sourceReplicas.value
    )
    gauge(
      builder,
      "storage_control_reconciliation_target_replicas",
      "Target replica count of the latest reconciliation.",
      operation.targetReplicas.value
    )
    gauge(
      builder,
      "storage_control_reconciliation_source_epoch",
      "Assignment epoch at the start of the latest reconciliation.",
      operation.sourceAssignmentEpoch.toDouble
    )
    operation.plannedAssignment.foreach(assignment =>
      gauge(
        builder,
        "storage_control_reconciliation_target_epoch",
        "Planned assignment epoch of the latest reconciliation.",
        assignment.epoch.toDouble
      )
    )
    gauge(
      builder,
      "storage_control_reconciliation_age_seconds",
      "Age of the latest reconciliation operation.",
      math.max(0, Duration.between(operation.createdAt, now).toMillis).toDouble / 1000
    )
    gauge(
      builder,
      "storage_control_reconciliation_duration_seconds",
      "Elapsed duration of the latest reconciliation operation.",
      math.max(0, Duration.between(operation.createdAt, operation.updatedAt).toMillis).toDouble / 1000
    )
    gauge(
      builder,
      "storage_control_reconciliation_waiting_for_router",
      "Whether the latest reconciliation is waiting for Router convergence.",
      boolean(operation.phase == ReconciliationPhase.WaitingForRouters)
    )
    if (operation.status == ReconciliationStatus.Succeeded)
      gauge(
        builder,
        "storage_control_reconciliation_last_success_timestamp_seconds",
        "Unix timestamp of the latest successful reconciliation.",
        operation.updatedAt.getEpochSecond.toDouble
      )
  }

  private def renderFleet(
      builder: StringBuilder,
      snapshot: ControllerSnapshot,
      fleet: Option[FleetSnapshot]
  ): Unit = fleet.foreach { observed =>
    gauge(
      builder,
      "storage_control_fleet_desired_replicas",
      "Desired replicas reported by the processing-unit fleet adapter.",
      observed.desiredReplicas.value
    )
    gauge(
      builder,
      "storage_control_fleet_observed_replicas",
      "Observed replicas reported by the processing-unit fleet adapter.",
      observed.observedReplicas.value
    )
    gauge(
      builder,
      "storage_control_fleet_ready_replicas",
      "Ready members reported by the processing-unit fleet adapter.",
      observed.readyMembers.size
    )
    snapshot.decision.foreach(decision =>
      gauge(
        builder,
        "storage_control_assignment_fleet_drift",
        "Absolute difference between authoritative assignment nodes and desired fleet replicas.",
        math.abs(decision.currentReplicas - observed.desiredReplicas.value)
      )
    )
  }

  private def renderSystemState(
      builder: StringBuilder,
      snapshot: ControllerSnapshot,
      config: ScalingConfig,
      now: Instant
  ): Unit = snapshot.systemState.foreach { state =>
    val expected = state.expectedNodes.size * SignalName.values.size
    val invalid =
      state.dataQuality.missingSignals.size + state.dataQuality.staleSignals.size + state.dataQuality.duplicateSignals.size
    val completeness = if (expected == 0) 0.0 else math.max(0.0, (expected - invalid).toDouble / expected)
    gauge(
      builder,
      "storage_control_data_completeness_ratio",
      "Fraction of required fresh unique node signals.",
      completeness
    )
    gauge(
      builder,
      "storage_control_missing_signals",
      "Number of required missing signals in the latest state.",
      state.dataQuality.missingSignals.size
    )
    gauge(
      builder,
      "storage_control_stale_signals",
      "Number of stale signals in the latest state.",
      state.dataQuality.staleSignals.size
    )
    gauge(
      builder,
      "storage_control_duplicate_signals",
      "Number of duplicate node signals in the latest state.",
      state.dataQuality.duplicateSignals.size
    )
    gauge(
      builder,
      "storage_control_unknown_nodes",
      "Number of metric node labels outside the active assignment.",
      state.dataQuality.unknownNodes.size
    )
    familyHeader(builder, "storage_control_signal_value", "Cluster value evaluated by scaling policy.")
    familyHeader(builder, "storage_control_signal_present", "Whether a cluster signal is available for policy.")
    familyHeader(builder, "storage_control_signal_age_seconds", "Age of each published cluster signal.")
    familyHeader(builder, "storage_control_signal_upper_threshold", "Configured scale-out threshold.")
    familyHeader(builder, "storage_control_signal_lower_threshold", "Configured scale-in threshold.")
    familyHeader(builder, "storage_control_signal_pressure_ratio", "Evaluated signal divided by its upper threshold.")
    SignalName.values.toList.sortBy(_.wireName).foreach { signal =>
      val labels = List("signal" -> signal.wireName, "unit" -> signal.unit.wireName)
      state.cluster.signals.get(signal) match {
        case Some(metric) =>
          sample(builder, "storage_control_signal_present", List("signal" -> signal.wireName), 1)
          sample(builder, "storage_control_signal_value", labels, metric.value)
          sample(
            builder,
            "storage_control_signal_age_seconds",
            List("signal" -> signal.wireName),
            math.max(0, Duration.between(metric.sampledAt, now).toMillis).toDouble / 1000
          )
        case None => sample(builder, "storage_control_signal_present", List("signal" -> signal.wireName), 0)
      }
    }
    SignalName.policySignals.toList.sortBy(_.wireName).foreach { signal =>
      val labels = List("signal" -> signal.wireName, "unit" -> signal.unit.wireName)
      val threshold = config.thresholds(signal)
      sample(builder, "storage_control_signal_upper_threshold", labels, threshold.upper)
      sample(builder, "storage_control_signal_lower_threshold", labels, threshold.lower)
      state.cluster.signals
        .get(signal)
        .foreach(metric =>
          sample(
            builder,
            "storage_control_signal_pressure_ratio",
            List("signal" -> signal.wireName),
            metric.value / threshold.upper
          )
        )
    }
    state.cluster.totalRequestsPerSecond.foreach(metric =>
      gauge(builder, "storage_control_cluster_requests_per_second", "Total observed processing-unit RPS.", metric.value)
    )
  }

  private def boolean(value: Boolean): Double = if (value) 1 else 0

  private def gauge(builder: StringBuilder, name: String, help: String, value: Double): Unit = {
    familyHeader(builder, name, help)
    sample(builder, name, Nil, value)
  }

  private def familyHeader(builder: StringBuilder, name: String, help: String, metricType: String = "gauge"): Unit = {
    builder.append(s"# HELP $name $help\n")
    builder.append(s"# TYPE $name $metricType\n")
  }

  private def sample(builder: StringBuilder, name: String, labels: List[(String, String)], value: Double): Unit = {
    val encodedLabels =
      if (labels.isEmpty) ""
      else labels.map { case (key, labelValue) => s"$key=\"$labelValue\"" }.mkString("{", ",", "}")
    builder.append(s"$name$encodedLabels $value\n")
  }
}
