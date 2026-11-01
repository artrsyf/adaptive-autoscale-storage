package assignmenthttp

import (
	"encoding/json"
	"net/http"
	"sort"

	"autoscale-distr-storage/router/internal/assignment/service"
	"autoscale-distr-storage/router/internal/assignment/transport/dto"
)

type AssignmentHttpHandler struct {
	AssignmentService *service.DynamicAssignmentService
}

func (handler *AssignmentHttpHandler) CurrentTopology(responseWriter http.ResponseWriter, httpRequest *http.Request) {
	snapshot, err := handler.AssignmentService.Snapshot()
	if err != nil {
		http.Error(responseWriter, `{"error":"topology_unavailable"}`, http.StatusServiceUnavailable)
		return
	}
	nodeIDs := make([]string, len(snapshot.Nodes))
	for nodeIndex, node := range snapshot.Nodes {
		nodeIDs[nodeIndex] = node.ID
	}
	sort.Strings(nodeIDs)
	responseWriter.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(responseWriter).Encode(dto.TopologyStatusResponse{
		Epoch:          snapshot.Epoch,
		PartitionCount: snapshot.PartitionCount,
		NodeIDs:        nodeIDs,
	})
}
