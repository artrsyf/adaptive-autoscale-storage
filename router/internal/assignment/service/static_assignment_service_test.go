package service

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	"testing"
)

// TestStablePartitionsAndOwnership проверяет эталонный хеш и независимость номера раздела от состава нод.
func TestStablePartitionsAndOwnership(test *testing.T) {
	threeNodeAssignment, _ := NewStaticAssignmentService(AssignmentConfig{Partitions: 128, Epoch: "1", Nodes: []assignment.AssignmentNode{{ID: "one", URL: "http://one:8080"}, {ID: "two", URL: "http://two:8080"}, {ID: "three", URL: "http://three:8080"}}})
	singleNodeAssignment, _ := NewStaticAssignmentService(AssignmentConfig{Partitions: 128, Epoch: "2", Nodes: threeNodeAssignment.config.Nodes[:1]})
	// SHA256("abc") starts ba7816bf8f01cfea: low seven bits are 106.
	if threeNodeAssignment.Partition("abc") != 106 {
		test.Fatalf("hash contract changed: %d", threeNodeAssignment.Partition("abc"))
	}
	for _, key := range []string{"abc", "tenant-1", "Москва"} {
		partitionID := threeNodeAssignment.Partition(key)
		if partitionID != singleNodeAssignment.Partition(key) {
			test.Fatal("partition depends on topology")
		}
		if threeNodeAssignment.Owner(partitionID).ID != threeNodeAssignment.config.Nodes[partitionID%3].ID {
			test.Fatal("wrong owner")
		}
	}
}

// TestRejectInvalidTopology проверяет отклонение пустой карты, повторных ID и неподдерживаемых адресов.
func TestRejectInvalidTopology(test *testing.T) {
	for _, nodes := range [][]assignment.AssignmentNode{nil, {{ID: "a", URL: "http://a"}, {ID: "a", URL: "http://b"}}, {{ID: "a", URL: "file:///tmp"}}, {{ID: "a", URL: "http://a/path"}}, {{ID: "", URL: "http://a"}}} {
		assignmentConfig := AssignmentConfig{Partitions: 128, Epoch: "1", Nodes: nodes}
		if assignmentConfig.Validate() == nil {
			test.Fatalf("accepted %v", nodes)
		}
	}
	if err := (&AssignmentConfig{Partitions: 128, Epoch: "1", Nodes: []assignment.AssignmentNode{{ID: "a", URL: "http://a"}}}).Validate(); err != nil {
		test.Fatal(err)
	}
}

// TestAssignmentSnapshot проверяет, что сервис не меняет маршруты при изменении исходного списка нод.
func TestAssignmentSnapshot(test *testing.T) {
	assignmentConfig := AssignmentConfig{Partitions: 128, Epoch: "1", Nodes: []assignment.AssignmentNode{{ID: "one", URL: "http://one"}}}
	staticAssignmentService, err := NewStaticAssignmentService(assignmentConfig)
	if err != nil {
		test.Fatal(err)
	}
	assignmentConfig.Nodes[0].ID = "changed"
	if staticAssignmentService.Resolve("abc").Node.ID != "one" {
		test.Fatal("assignment depends on mutable config")
	}
	if _, err := NewStaticAssignmentService(AssignmentConfig{}); err == nil {
		test.Fatal("invalid assignment accepted")
	}
}
