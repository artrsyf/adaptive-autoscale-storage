package storage.controlplane.reconciliation.domain

final case class RouterTopologyState(epoch: Long, partitionCount: Int, nodeIds: Set[String]) {
  def validate: Either[String, RouterTopologyState] =
    if (epoch <= 0 || partitionCount < 1 || partitionCount > 1024 || nodeIds.isEmpty)
      Left("invalid Router topology dimensions")
    else if (nodeIds.exists(_.trim.isEmpty)) Left("Router topology contains an empty node identifier")
    else Right(this)
}

sealed abstract class RouterTopologyError(message: String, cause: Throwable = null) extends Exception(message, cause)

object RouterTopologyError {
  final case class InvalidData(details: String) extends RouterTopologyError(details)
  final case class Protocol(details: String) extends RouterTopologyError(details)
  final case class Unavailable(underlying: Throwable)
      extends RouterTopologyError("Router topology unavailable", underlying)
}
