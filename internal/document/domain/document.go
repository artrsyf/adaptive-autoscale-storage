// Package document defines document values and invariants without infrastructure dependencies.
package document

import (
	"bytes"
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

type Key struct{ PartitionKey, ID string }

// Payload is a JSON object by domain contract; it has no HTTP or SQL encoding tags.
type Payload []byte
type Record struct {
	Key
	Payload              Payload
	Revision             string
	CreatedAt, UpdatedAt time.Time
}

func (k Key) Validate() error {
	if strings.TrimSpace(k.PartitionKey) == "" || strings.TrimSpace(k.ID) == "" || len(k.PartitionKey) > 256 || len(k.ID) > 256 || strings.ContainsRune(k.PartitionKey+k.ID, '\x00') {
		return ErrInvalid
	}
	return nil
}
func (p Payload) Validate() error {
	b := bytes.TrimSpace(p)
	if len(b) == 0 || b[0] != '{' || !json.Valid(b) {
		return ErrInvalid
	}
	return nil
}
func ValidateRevision(revision string) error {
	if revision == "" || len(revision) > 128 {
		return ErrInvalid
	}
	return nil
}
