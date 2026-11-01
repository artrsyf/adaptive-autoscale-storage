package assignmenthttp

import (
	"net/http"

	"autoscale-distr-storage/router/internal/assignment/service"
)

func NewAssignmentHttpRoutes(assignmentService *service.DynamicAssignmentService) http.Handler {
	httpMux := http.NewServeMux()
	handler := &AssignmentHttpHandler{AssignmentService: assignmentService}
	httpMux.HandleFunc("GET /topology", handler.CurrentTopology)
	return httpMux
}
