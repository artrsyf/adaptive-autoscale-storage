package assignment

type PartitionOwner struct {
	Partition int    `json:"partition"`
	NodeID    string `json:"node_id"`
}

type AssignmentSnapshot struct {
	Epoch          uint64           `json:"epoch"`
	PartitionCount int              `json:"partition_count"`
	Nodes          []AssignmentNode `json:"nodes"`
	Assignments    []PartitionOwner `json:"assignments"`
}
