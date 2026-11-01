package storage.controlplane.reconciliation.service

import storage.controlplane.assignment.service.AssignmentService
import storage.controlplane.reconciliation.domain.{ActuationConfig, ActuationMode}
import storage.controlplane.scaling.domain.ScalingDecisionKind
import storage.controlplane.scaling.service.ControllerService
import zio.{Clock, Duration as ZioDuration, UIO, URLayer, ZIO, ZLayer}

final class ActuationController(
    config: ActuationConfig,
    controller: ControllerService,
    assignments: AssignmentService,
    operations: ScalingOperationService,
    reconciler: TopologyReconciler
) {
  def run: UIO[Nothing] =
    (runOnce.ignore *> ZIO.sleep(ZioDuration.fromMillis(config.reconcileInterval.toMillis))).forever

  private[service] def runOnce =
    if (config.mode == ActuationMode.Advisory) ZIO.unit
    else
      for {
        active <- operations.active
        _ <- active match {
          case Some(_) => ZIO.unit
          case None =>
            controller.snapshot.flatMap { snapshot =>
              snapshot.decision match {
                case Some(decision)
                    if decision.kind == ScalingDecisionKind.ScaleOut ||
                      decision.kind == ScalingDecisionKind.ScaleIn =>
                  for {
                    assignment <- assignments.current
                    startedAt <- Clock.instant
                    _ <- operations.start(decision, assignment, startedAt)
                  } yield ()
                case _ => ZIO.unit
              }
            }
        }
        evaluatedAt <- Clock.instant
        _ <- reconciler.runOnce(evaluatedAt)
      } yield ()
}

object ActuationController {
  val layer: URLayer[
    ActuationConfig & ControllerService & AssignmentService & ScalingOperationService & TopologyReconciler,
    ActuationController
  ] = ZLayer.derive[ActuationController]
}
