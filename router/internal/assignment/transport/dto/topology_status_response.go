package dto

type TopologyStatusResponse struct {
	Epoch          uint64   `json:"epoch"`
	PartitionCount int      `json:"partition_count"`
	NodeIDs        []string `json:"node_ids"`
}
