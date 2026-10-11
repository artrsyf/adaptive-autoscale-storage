package storage.controlplane.assignment.service

import storage.controlplane.assignment.domain.AssignmentError.EpochConflict
import storage.controlplane.assignment.domain.{AssignmentError, AssignmentSnapshot, Node, PartitionOwner}
import zio.test._
import zio.{IO, Ref, ZIO}

object AssignmentServiceSpec extends ZIOSpecDefault {
  private val initial = AssignmentSnapshot(
    1,
    1,
    List(Node("one", "http://one:8080")),
    List(PartitionOwner(0, "one"))
  )

  private final class StubRepository(state: Ref[AssignmentSnapshot], failure: Option[AssignmentError])
      extends AssignmentRepository {
    override def current: IO[AssignmentError, AssignmentSnapshot] = failure.fold(state.get)(ZIO.fail(_))

    override def publish(expectedEpoch: Long, snapshot: AssignmentSnapshot): IO[AssignmentError, Unit] =
      failure.fold(
        state.modify { current =>
          if (current.epoch != expectedEpoch) ZIO.fail(EpochConflict) -> current
          else ZIO.unit -> snapshot
        }.flatten
      )(ZIO.fail(_))

    override def ready: IO[AssignmentError, Unit] = failure.fold[IO[AssignmentError, Unit]](ZIO.unit)(ZIO.fail(_))
  }

  override def spec = suite("AssignmentService")(
    test("publishes only the next complete epoch") {
      for {
        state <- Ref.make(initial)
        service <- AssignmentService.make(new StubRepository(state, None))
        next = initial.copy(epoch = 2)
        published <- service.publish(1, next)
        activeEpoch <- service.epoch
        invalid <- service.publish(2, next.copy(epoch = 4)).either
      } yield assertTrue(published == next, activeEpoch == 2, invalid.isLeft)
    },
    test("preserves repository epoch conflicts") {
      for {
        state <- Ref.make(initial)
        service <- AssignmentService.make(new StubRepository(state, Some(EpochConflict)))
        result <- service.publish(1, initial.copy(epoch = 2)).either
      } yield assertTrue(result == Left(EpochConflict))
    }
  )
}
