package document

import "strings"

type Key struct{ PartitionKey, ID string }

// Validate отклоняет пустые ключи, NUL и ключи длиннее 256 байт.
func (documentKey Key) Validate() error {
	if strings.TrimSpace(documentKey.PartitionKey) == "" || strings.TrimSpace(documentKey.ID) == "" || len(documentKey.PartitionKey) > 256 || len(documentKey.ID) > 256 || strings.ContainsRune(documentKey.PartitionKey+documentKey.ID, '\x00') {
		return ErrInvalid
	}
	return nil
}
