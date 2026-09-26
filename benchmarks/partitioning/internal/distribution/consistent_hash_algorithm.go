package distribution

import (
	"fmt"
	"sort"
)

type ConsistentHashAlgorithm struct {
	virtualNodes int
}

func NewConsistentHashAlgorithm(virtualNodes int) (ConsistentHashAlgorithm, error) {
	if virtualNodes < 1 {
		return ConsistentHashAlgorithm{}, fmt.Errorf("virtual node count must be positive")
	}
	return ConsistentHashAlgorithm{virtualNodes: virtualNodes}, nil
}

func (ConsistentHashAlgorithm) Name() string {
	return "consistent_hash"
}

func (consistentHashAlgorithm ConsistentHashAlgorithm) Build(buildRequest BuildRequest) (Placement, error) {
	if err := validateNodes(buildRequest.Nodes); err != nil {
		return nil, err
	}
	ringPoints := make([]ringPoint, 0, len(buildRequest.Nodes)*consistentHashAlgorithm.virtualNodes)
	for _, nodeID := range buildRequest.Nodes {
		for virtualNodeIndex := 0; virtualNodeIndex < consistentHashAlgorithm.virtualNodes; virtualNodeIndex++ {
			ringPoints = append(ringPoints, ringPoint{
				token: hashValue(fmt.Sprintf("%s#%d", nodeID, virtualNodeIndex)),
				node:  nodeID,
			})
		}
	}
	sort.Slice(ringPoints, func(leftIndex, rightIndex int) bool {
		if ringPoints[leftIndex].token == ringPoints[rightIndex].token {
			return ringPoints[leftIndex].node < ringPoints[rightIndex].node
		}
		return ringPoints[leftIndex].token < ringPoints[rightIndex].token
	})
	return &consistentHashPlacement{ringPoints: ringPoints}, nil
}

type ringPoint struct {
	token uint64
	node  NodeID
}

type consistentHashPlacement struct {
	ringPoints []ringPoint
}

func (consistentHashPlacement *consistentHashPlacement) Owner(key string) NodeID {
	keyToken := hashValue(key)
	pointIndex := sort.Search(len(consistentHashPlacement.ringPoints), func(pointIndex int) bool {
		return consistentHashPlacement.ringPoints[pointIndex].token >= keyToken
	})
	if pointIndex == len(consistentHashPlacement.ringPoints) {
		pointIndex = 0
	}
	return consistentHashPlacement.ringPoints[pointIndex].node
}

func (*consistentHashPlacement) Metadata() PlacementMetadata {
	return PlacementMetadata{}
}
