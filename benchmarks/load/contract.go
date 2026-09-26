package main

import (
	"autoscale-distr-storage-benchmarks/transport/dto"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// partition повторяет контракт хеширования Router для выбора ключей горячего раздела в нагрузке.
func partition(key string, partitions int) int {
	hash := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(hash[:8]) % uint64(partitions))
}

// decodeDocumentRevision читает revision из DTO ответа конкретной операции.
func decodeDocumentRevision(decoder *json.Decoder, operation string) (string, error) {
	switch operation {
	case "create":
		var response dto.DocumentCreateResponse
		err := decoder.Decode(&response)
		return response.Revision, err
	case "get":
		var response dto.DocumentGetResponse
		err := decoder.Decode(&response)
		return response.Revision, err
	case "update":
		var response dto.DocumentUpdateResponse
		err := decoder.Decode(&response)
		return response.Revision, err
	default:
		return "", fmt.Errorf("unsupported document operation %q", operation)
	}
}
