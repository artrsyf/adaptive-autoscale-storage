package distribution

import "fmt"

type LogicalPartitionAlgorithm struct {
	partitionCount int
}

func NewLogicalPartitionAlgorithm(partitionCount int) (LogicalPartitionAlgorithm, error) {
	if partitionCount < 1 {
		return LogicalPartitionAlgorithm{}, fmt.Errorf("logical partition count must be positive")
	}
	return LogicalPartitionAlgorithm{partitionCount: partitionCount}, nil
}

func (LogicalPartitionAlgorithm) Name() string {
	return "logical_partitions"
}

func (logicalPartitionAlgorithm LogicalPartitionAlgorithm) Build(buildRequest BuildRequest) (Placement, error) {
	if err := validateNodes(buildRequest.Nodes); err != nil {
		return nil, err
	}
	if buildRequest.Previous == nil {
		return logicalPartitionAlgorithm.initialPlacement(buildRequest.Nodes), nil
	}
	previousPlacement, validType := buildRequest.Previous.(*logicalPartitionPlacement)
	if !validType || len(previousPlacement.owners) != logicalPartitionAlgorithm.partitionCount {
		return nil, fmt.Errorf("previous placement was not produced by the same logical partition algorithm")
	}
	return logicalPartitionAlgorithm.rebalancedPlacement(previousPlacement, buildRequest.Nodes), nil
}

func (logicalPartitionAlgorithm LogicalPartitionAlgorithm) initialPlacement(nodes []NodeID) Placement {
	owners := make([]NodeID, logicalPartitionAlgorithm.partitionCount)
	for partitionID := range owners {
		owners[partitionID] = nodes[partitionID%len(nodes)]
	}
	return &logicalPartitionPlacement{owners: owners}
}

func (logicalPartitionAlgorithm LogicalPartitionAlgorithm) rebalancedPlacement(previousPlacement *logicalPartitionPlacement, nodes []NodeID) Placement {
	targetCounts := make(map[NodeID]int, len(nodes))
	baseCount := logicalPartitionAlgorithm.partitionCount / len(nodes)
	extraPartitions := logicalPartitionAlgorithm.partitionCount % len(nodes)
	for nodeIndex, nodeID := range nodes {
		targetCounts[nodeID] = baseCount
		if nodeIndex < extraPartitions {
			targetCounts[nodeID]++
		}
	}

	owners := make([]NodeID, logicalPartitionAlgorithm.partitionCount)
	retainedCounts := make(map[NodeID]int, len(nodes))
	unassignedPartitions := make([]int, 0, logicalPartitionAlgorithm.partitionCount)
	for partitionID, previousOwner := range previousPlacement.owners {
		targetCount, remainsInTopology := targetCounts[previousOwner]
		if remainsInTopology && retainedCounts[previousOwner] < targetCount {
			owners[partitionID] = previousOwner
			retainedCounts[previousOwner]++
			continue
		}
		unassignedPartitions = append(unassignedPartitions, partitionID)
	}

	unassignedIndex := 0
	for _, nodeID := range nodes {
		for retainedCounts[nodeID] < targetCounts[nodeID] {
			owners[unassignedPartitions[unassignedIndex]] = nodeID
			unassignedIndex++
			retainedCounts[nodeID]++
		}
	}
	return &logicalPartitionPlacement{owners: owners}
}

type logicalPartitionPlacement struct {
	owners []NodeID
}

func (logicalPartitionPlacement *logicalPartitionPlacement) Owner(key string) NodeID {
	partitionID := hashValue(key) % uint64(len(logicalPartitionPlacement.owners))
	return logicalPartitionPlacement.owners[partitionID]
}

func (logicalPartitionPlacement *logicalPartitionPlacement) Metadata() PlacementMetadata {
	assignments := make([]UnitAssignment, len(logicalPartitionPlacement.owners))
	for partitionID, nodeID := range logicalPartitionPlacement.owners {
		assignments[partitionID] = UnitAssignment{Unit: partitionID, Owner: nodeID}
	}
	return PlacementMetadata{UnitAssignments: assignments}
}
