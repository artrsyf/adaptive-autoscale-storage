package service

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"net/url"
	"strings"
	"sync/atomic"

	"autoscale-distr-storage/router/internal/assignment/domain"
)

type installedSnapshot struct {
	epoch          uint64
	partitionCount int
	nodes          []assignment.AssignmentNode
	owners         []assignment.AssignmentNode
}

type DynamicAssignmentService struct {
	snapshot atomic.Pointer[installedSnapshot]
}

func NewDynamicAssignmentService() *DynamicAssignmentService { return &DynamicAssignmentService{} }

func (assignmentService *DynamicAssignmentService) Install(snapshot assignment.AssignmentSnapshot) error {
	if snapshot.Epoch == 0 || snapshot.Epoch > math.MaxInt64 || snapshot.PartitionCount < 1 || snapshot.PartitionCount > 1024 || len(snapshot.Nodes) == 0 || len(snapshot.Assignments) != snapshot.PartitionCount {
		return fmt.Errorf("invalid assignment snapshot dimensions")
	}
	nodesByID := make(map[string]assignment.AssignmentNode, len(snapshot.Nodes))
	nodes := append([]assignment.AssignmentNode(nil), snapshot.Nodes...)
	for _, node := range nodes {
		parsedURL, err := url.Parse(node.URL)
		if strings.TrimSpace(node.ID) == "" || nodesByID[node.ID].ID != "" || err != nil || parsedURL.Scheme != "http" || parsedURL.Host == "" || parsedURL.Path != "" || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || parsedURL.User != nil {
			return fmt.Errorf("invalid assignment node")
		}
		nodesByID[node.ID] = node
	}
	owners := make([]assignment.AssignmentNode, snapshot.PartitionCount)
	assigned := make([]bool, snapshot.PartitionCount)
	for _, partitionOwner := range snapshot.Assignments {
		node, exists := nodesByID[partitionOwner.NodeID]
		if partitionOwner.Partition < 0 || partitionOwner.Partition >= snapshot.PartitionCount || assigned[partitionOwner.Partition] || !exists {
			return fmt.Errorf("invalid partition owner")
		}
		assigned[partitionOwner.Partition] = true
		owners[partitionOwner.Partition] = node
	}
	current := assignmentService.snapshot.Load()
	if current != nil && snapshot.Epoch < current.epoch {
		return fmt.Errorf("assignment epoch regressed")
	}
	if current != nil && snapshot.Epoch == current.epoch {
		return nil
	}
	assignmentService.snapshot.Store(&installedSnapshot{epoch: snapshot.Epoch, partitionCount: snapshot.PartitionCount, nodes: nodes, owners: owners})
	return nil
}

func (assignmentService *DynamicAssignmentService) Resolve(key string) (assignment.AssignmentRoute, error) {
	snapshot := assignmentService.snapshot.Load()
	if snapshot == nil {
		return assignment.AssignmentRoute{}, fmt.Errorf("assignment snapshot unavailable")
	}
	keyHash := sha256.Sum256([]byte(key))
	partitionID := int(binary.BigEndian.Uint64(keyHash[:8]) % uint64(snapshot.partitionCount))
	return assignment.AssignmentRoute{Partition: partitionID, Node: snapshot.owners[partitionID], Epoch: snapshot.epoch}, nil
}

func (assignmentService *DynamicAssignmentService) Nodes() []assignment.AssignmentNode {
	snapshot := assignmentService.snapshot.Load()
	if snapshot == nil {
		return nil
	}
	return append([]assignment.AssignmentNode(nil), snapshot.nodes...)
}

func (assignmentService *DynamicAssignmentService) Epoch() uint64 {
	if snapshot := assignmentService.snapshot.Load(); snapshot != nil {
		return snapshot.epoch
	}
	return 0
}
