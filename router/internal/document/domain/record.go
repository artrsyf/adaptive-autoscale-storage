package document

import "time"

type Record struct {
	Key
	Payload              Payload
	Revision             string
	CreatedAt, UpdatedAt time.Time
}
