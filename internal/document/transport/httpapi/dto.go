package api

import (
	"context"
	"encoding/json"
	"time"

	document "autoscale-distr-storage/internal/document/domain"
)

type Request struct {
	PartitionKey     string          `json:"partition_key"`
	ID               string          `json:"id"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	ExpectedRevision string          `json:"expected_revision,omitempty"`
}

type Response struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Revision     string          `json:"revision"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// Executor is consumed by the HTTP handler and implemented by the local and
// forwarding adapters. Domain and persistence do not depend on this port.
type Executor interface {
	Execute(context.Context, string, Request) (Response, error)
}

func validOperation(op string) bool {
	return op == "create" || op == "get" || op == "update" || op == "delete"
}

func (r Request) key() document.Key { return document.Key{PartitionKey: r.PartitionKey, ID: r.ID} }

func (r Request) validate(op string) error {
	if !validOperation(op) {
		return document.ErrInvalid
	}
	if err := r.key().Validate(); err != nil {
		return err
	}
	if op == "create" || op == "update" {
		if err := document.Payload(r.Payload).Validate(); err != nil {
			return err
		}
	} else if len(r.Payload) != 0 {
		return document.ErrInvalid
	}
	if op == "update" || op == "delete" {
		return document.ValidateRevision(r.ExpectedRevision)
	}
	if r.ExpectedRevision != "" {
		return document.ErrInvalid
	}
	return nil
}
