package service

import (
	"testing"

	"autoscale-distr-storage/router/internal/assignment/domain"
)

func TestDynamicSnapshotRouting(test *testing.T) {
	assignmentService := NewDynamicAssignmentService()
	if _, err := assignmentService.Resolve("abc"); err == nil {
		test.Fatal("resolved without snapshot")
	}
	snapshot := assignment.AssignmentSnapshot{Epoch: 7, PartitionCount: 128, Nodes: []assignment.AssignmentNode{{ID: "one", URL: "http://one:8080"}, {ID: "two", URL: "http://two:8080"}}, Assignments: make([]assignment.PartitionOwner, 128)}
	for partitionID := range snapshot.Assignments {
		nodeID := "one"
		if partitionID%2 == 1 {
			nodeID = "two"
		}
		snapshot.Assignments[partitionID] = assignment.PartitionOwner{Partition: partitionID, NodeID: nodeID}
	}
	if err := assignmentService.Install(snapshot); err != nil {
		test.Fatal(err)
	}
	route, err := assignmentService.Resolve("abc")
	if err != nil || route.Partition != 106 || route.Node.ID != "one" || route.Epoch != 7 {
		test.Fatalf("wrong route: %+v %v", route, err)
	}
	if err = assignmentService.Install(assignment.AssignmentSnapshot{Epoch: 6}); err == nil {
		test.Fatal("accepted invalid or regressed snapshot")
	}
}
