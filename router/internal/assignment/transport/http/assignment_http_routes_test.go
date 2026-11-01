package assignmenthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	assignment "autoscale-distr-storage/router/internal/assignment/domain"
	"autoscale-distr-storage/router/internal/assignment/service"
	"autoscale-distr-storage/router/internal/assignment/transport/dto"
)

func TestCurrentTopology(test *testing.T) {
	assignmentService := service.NewDynamicAssignmentService()
	snapshot := assignment.AssignmentSnapshot{
		Epoch:          7,
		PartitionCount: 2,
		Nodes: []assignment.AssignmentNode{
			{ID: "processing-unit-2", URL: "http://processing-unit-2:8080"},
			{ID: "processing-unit-1", URL: "http://processing-unit-1:8080"},
		},
		Assignments: []assignment.PartitionOwner{
			{Partition: 0, NodeID: "processing-unit-1"},
			{Partition: 1, NodeID: "processing-unit-2"},
		},
	}
	if err := assignmentService.Install(snapshot); err != nil {
		test.Fatal(err)
	}
	httpRequest := httptest.NewRequest(http.MethodGet, "/topology", nil)
	responseRecorder := httptest.NewRecorder()
	NewAssignmentHttpRoutes(assignmentService).ServeHTTP(responseRecorder, httpRequest)
	var response dto.TopologyStatusResponse
	if err := json.NewDecoder(responseRecorder.Body).Decode(&response); err != nil {
		test.Fatal(err)
	}
	if responseRecorder.Code != http.StatusOK || response.Epoch != 7 || response.PartitionCount != 2 ||
		len(response.NodeIDs) != 2 || response.NodeIDs[0] != "processing-unit-1" || response.NodeIDs[1] != "processing-unit-2" {
		test.Fatalf("unexpected response: status=%d body=%+v", responseRecorder.Code, response)
	}
}
