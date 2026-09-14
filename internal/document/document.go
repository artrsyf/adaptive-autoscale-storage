// Package document defines transport-independent document commands.
package document

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound = errors.New("not_found")
	ErrConflict = errors.New("revision_conflict")
	ErrExists   = errors.New("already_exists")
	ErrInvalid  = errors.New("invalid_request")
)

type Command struct {
	PartitionKey     string          `json:"partition_key"`
	ID               string          `json:"id"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	ExpectedRevision string          `json:"expected_revision,omitempty"`
}

type Record struct {
	PartitionKey string          `json:"partition_key"`
	ID           string          `json:"id"`
	Payload      json.RawMessage `json:"payload,omitempty"`
	Revision     string          `json:"revision"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type Executor interface {
	Execute(context.Context, string, Command) (Record, error)
}

func ValidOperation(op string) bool {
	return op == "create" || op == "get" || op == "update" || op == "delete"
}

func (c Command) Validate(op string) error {
	if !ValidOperation(op) || strings.TrimSpace(c.PartitionKey) == "" || strings.TrimSpace(c.ID) == "" || len(c.PartitionKey) > 256 || len(c.ID) > 256 || strings.ContainsRune(c.PartitionKey+c.ID, '\x00') {
		return ErrInvalid
	}
	if op == "create" || op == "update" {
		p := bytes.TrimSpace(c.Payload)
		if len(p) == 0 || p[0] != '{' || !json.Valid(p) {
			return ErrInvalid
		}
	} else if len(c.Payload) != 0 {
		return ErrInvalid
	}
	if op == "update" || op == "delete" {
		if c.ExpectedRevision == "" || len(c.ExpectedRevision) > 128 {
			return ErrInvalid
		}
	} else if c.ExpectedRevision != "" {
		return ErrInvalid
	}
	return nil
}
