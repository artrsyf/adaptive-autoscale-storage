package assignment

// AssignmentRoute describes a resolved processing destination, not an HTTP request.
type AssignmentRoute struct {
	Partition int
	Node      AssignmentNode
	Epoch     string
}
