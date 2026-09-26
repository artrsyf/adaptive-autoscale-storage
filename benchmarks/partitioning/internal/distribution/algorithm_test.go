package distribution

import (
	"strconv"
	"testing"
)

func TestAlgorithmsProduceDeterministicOwners(test *testing.T) {
	consistentHashAlgorithm, err := NewConsistentHashAlgorithm(32)
	if err != nil {
		test.Fatal(err)
	}
	logicalPartitionAlgorithm, err := NewLogicalPartitionAlgorithm(128)
	if err != nil {
		test.Fatal(err)
	}
	algorithms := []DistributionAlgorithm{
		NewModuloAlgorithm(),
		consistentHashAlgorithm,
		logicalPartitionAlgorithm,
	}
	nodes := nodeIDs(3)
	for _, algorithm := range algorithms {
		firstPlacement, operationError := algorithm.Build(BuildRequest{Nodes: nodes})
		if operationError != nil {
			test.Fatalf("%s: %v", algorithm.Name(), operationError)
		}
		secondPlacement, operationError := algorithm.Build(BuildRequest{Nodes: nodes})
		if operationError != nil {
			test.Fatalf("%s: %v", algorithm.Name(), operationError)
		}
		for _, key := range []string{"abc", "tenant-1", "Москва", "key/42/999"} {
			if firstPlacement.Owner(key) != secondPlacement.Owner(key) {
				test.Fatalf("%s returned a non-deterministic owner for %q", algorithm.Name(), key)
			}
		}
	}
}

func TestLogicalPartitionRebalanceMinimizesMovement(test *testing.T) {
	algorithm, err := NewLogicalPartitionAlgorithm(128)
	if err != nil {
		test.Fatal(err)
	}
	threeNodePlacement, err := algorithm.Build(BuildRequest{Nodes: nodeIDs(3)})
	if err != nil {
		test.Fatal(err)
	}
	fiveNodePlacement, err := algorithm.Build(BuildRequest{Nodes: nodeIDs(5), Previous: threeNodePlacement})
	if err != nil {
		test.Fatal(err)
	}

	if changedAssignments(threeNodePlacement, fiveNodePlacement) != 50 {
		test.Fatalf("3 to 5 should move the 50 partitions needed by new nodes, got %d", changedAssignments(threeNodePlacement, fiveNodePlacement))
	}
	assertBalancedAndComplete(test, fiveNodePlacement, nodeIDs(5), 128)

	backToThreePlacement, err := algorithm.Build(BuildRequest{Nodes: nodeIDs(3), Previous: fiveNodePlacement})
	if err != nil {
		test.Fatal(err)
	}
	if changedAssignments(fiveNodePlacement, backToThreePlacement) != 50 {
		test.Fatalf("5 to 3 should only move partitions from removed nodes, got %d", changedAssignments(fiveNodePlacement, backToThreePlacement))
	}
	assertBalancedAndComplete(test, backToThreePlacement, nodeIDs(3), 128)
}

func TestAlgorithmsRejectInvalidConfiguration(test *testing.T) {
	if _, err := NewConsistentHashAlgorithm(0); err == nil {
		test.Fatal("accepted zero virtual nodes")
	}
	if _, err := NewLogicalPartitionAlgorithm(0); err == nil {
		test.Fatal("accepted zero logical partitions")
	}
	for _, nodes := range [][]NodeID{nil, {""}, {"node-1", "node-1"}} {
		if _, err := NewModuloAlgorithm().Build(BuildRequest{Nodes: nodes}); err == nil {
			test.Fatalf("accepted invalid nodes %v", nodes)
		}
	}
}

func TestHashContractMatchesRouter(test *testing.T) {
	if hashValue("abc")%128 != 106 {
		test.Fatalf("hash contract changed: got partition %d", hashValue("abc")%128)
	}
}

func changedAssignments(previousPlacement Placement, nextPlacement Placement) int {
	previousAssignments := previousPlacement.Metadata().UnitAssignments
	nextAssignments := nextPlacement.Metadata().UnitAssignments
	changedCount := 0
	for assignmentIndex := range previousAssignments {
		if previousAssignments[assignmentIndex].Owner != nextAssignments[assignmentIndex].Owner {
			changedCount++
		}
	}
	return changedCount
}

func assertBalancedAndComplete(test *testing.T, placement Placement, nodes []NodeID, expectedPartitions int) {
	test.Helper()
	assignments := placement.Metadata().UnitAssignments
	if len(assignments) != expectedPartitions {
		test.Fatalf("expected %d assignments, got %d", expectedPartitions, len(assignments))
	}
	counts := make(map[NodeID]int, len(nodes))
	for partitionID, assignment := range assignments {
		if assignment.Unit != partitionID || assignment.Owner == "" {
			test.Fatalf("invalid assignment at partition %d: %+v", partitionID, assignment)
		}
		counts[assignment.Owner]++
	}
	minimumCount, maximumCount := expectedPartitions, 0
	for _, nodeID := range nodes {
		if counts[nodeID] < minimumCount {
			minimumCount = counts[nodeID]
		}
		if counts[nodeID] > maximumCount {
			maximumCount = counts[nodeID]
		}
	}
	if maximumCount-minimumCount > 1 {
		test.Fatalf("unbalanced assignments: %v", counts)
	}
}

func nodeIDs(count int) []NodeID {
	nodes := make([]NodeID, count)
	for nodeIndex := range nodes {
		nodes[nodeIndex] = NodeID("node-" + strconv.Itoa(nodeIndex+1))
	}
	return nodes
}
