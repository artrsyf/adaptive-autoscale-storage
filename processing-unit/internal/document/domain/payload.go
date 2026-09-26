package document

import (
	"bytes"
	"encoding/json"
)

// Payload is a JSON object by domain contract; it has no HTTP or SQL encoding tags.
type Payload []byte

// Validate проверяет, что содержимое является корректным JSON-объектом.
func (documentPayload Payload) Validate() error {
	trimmedPayload := bytes.TrimSpace(documentPayload)
	if len(trimmedPayload) == 0 || trimmedPayload[0] != '{' || !json.Valid(trimmedPayload) {
		return ErrInvalid
	}
	return nil
}
