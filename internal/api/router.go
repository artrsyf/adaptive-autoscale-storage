package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"autoscale-distr-storage/internal/document"
	"autoscale-distr-storage/internal/topology"
)

type UpstreamError struct {
	Status int
	Code   string
}

func (e *UpstreamError) Error() string { return fmt.Sprintf("upstream %d: %s", e.Status, e.Code) }

type Router struct {
	Assignment *topology.Assignment
	Client     *http.Client
}

func (r *Router) Execute(ctx context.Context, op string, c document.Command) (document.Record, error) {
	var record document.Record
	data, err := json.Marshal(c)
	if err != nil {
		return record, err
	}
	node := r.Assignment.Owner(r.Assignment.Partition(c.PartitionKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, node.URL+"/"+op, bytes.NewReader(data))
	if err != nil {
		return record, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Assignment-Epoch", r.Assignment.Epoch)
	resp, err := r.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return record, ctx.Err()
		}
		return record, &UpstreamError{502, "pu_unavailable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e struct {
			Error string `json:"error"`
		}
		if json.NewDecoder(io.LimitReader(resp.Body, 4096)).Decode(&e) != nil {
			return record, &UpstreamError{502, "invalid_upstream_response"}
		}
		return record, &UpstreamError{resp.StatusCode, e.Error}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&record); err != nil {
		return record, &UpstreamError{502, "invalid_upstream_response"}
	}
	return record, nil
}
