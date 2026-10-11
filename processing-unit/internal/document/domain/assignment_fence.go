package document

// AssignmentFence is the ownership precondition that persistence must validate atomically with a command.
type AssignmentFence struct {
	Partition int
	NodeID    string
	Epoch     uint64
}
