package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
)

// HTTP DTOs belong to the external workload client; no application imports.
type commandDTO struct {
	PartitionKey     string          `json:"partition_key"`
	ID               string          `json:"id"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	ExpectedRevision string          `json:"expected_revision,omitempty"`
}
type recordDTO struct {
	Revision string `json:"revision"`
}

// Mirrors the documented partition contract for targeted hot-partition workloads.
func partition(key string) int {
	hash := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(hash[:8]) % 128)
}
