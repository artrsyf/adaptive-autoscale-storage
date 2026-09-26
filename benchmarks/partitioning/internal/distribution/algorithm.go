// Package distribution defines topology-independent placement algorithms used by the research harness.
package distribution

import "fmt"

type NodeID string

type UnitAssignment struct {
	Unit  int    `json:"unit"`
	Owner NodeID `json:"owner"`
}

type PlacementMetadata struct {
	// UnitAssignments describes stable movable units when an algorithm exposes them.
	// An empty slice means that movement is measured only at key level.
	UnitAssignments []UnitAssignment `json:"unit_assignments,omitempty"`
}

// Placement is an immutable routing snapshot produced for one topology.
type Placement interface {
	Owner(key string) NodeID
	Metadata() PlacementMetadata
}

type BuildRequest struct {
	Nodes    []NodeID
	Previous Placement
}

// DistributionAlgorithm builds an initial placement and a subsequent placement from the same contract.
// Implementations may use Previous to minimize movement, but callers do not depend on their internals.
type DistributionAlgorithm interface {
	Name() string
	Build(BuildRequest) (Placement, error)
}

func validateNodes(nodes []NodeID) error {
	if len(nodes) == 0 {
		return fmt.Errorf("topology must contain at least one node")
	}
	seenNodes := make(map[NodeID]struct{}, len(nodes))
	for _, nodeID := range nodes {
		if nodeID == "" {
			return fmt.Errorf("node ID must not be empty")
		}
		if _, exists := seenNodes[nodeID]; exists {
			return fmt.Errorf("duplicate node ID %q", nodeID)
		}
		seenNodes[nodeID] = struct{}{}
	}
	return nil
}
