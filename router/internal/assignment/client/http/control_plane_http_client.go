package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"autoscale-distr-storage/router/internal/assignment/domain"
)

type ControlPlaneHttpClient struct {
	BaseURL string
	Client  *http.Client
}

func (client *ControlPlaneHttpClient) Current(operationContext context.Context) (assignment.AssignmentSnapshot, error) {
	httpRequest, err := http.NewRequestWithContext(operationContext, http.MethodGet, client.BaseURL+"/assignments/current", nil)
	if err != nil {
		return assignment.AssignmentSnapshot{}, err
	}
	httpResponse, err := client.Client.Do(httpRequest)
	if err != nil {
		return assignment.AssignmentSnapshot{}, err
	}
	defer httpResponse.Body.Close()
	if httpResponse.StatusCode != http.StatusOK {
		return assignment.AssignmentSnapshot{}, fmt.Errorf("control plane returned status %d", httpResponse.StatusCode)
	}
	var snapshot assignment.AssignmentSnapshot
	decoder := json.NewDecoder(io.LimitReader(httpResponse.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&snapshot); err != nil {
		return assignment.AssignmentSnapshot{}, err
	}
	if err = decoder.Decode(new(any)); err != io.EOF {
		return assignment.AssignmentSnapshot{}, fmt.Errorf("control plane returned multiple JSON values")
	}
	return snapshot, nil
}
