package distribution

import (
	"crypto/sha256"
	"encoding/binary"
)

// hashValue matches the Router contract: first eight SHA-256 bytes as unsigned big-endian.
func hashValue(value string) uint64 {
	valueHash := sha256.Sum256([]byte(value))
	return binary.BigEndian.Uint64(valueHash[:8])
}
