package api

import (
	"context"
	"encoding/json"

	document "autoscale-distr-storage/internal/document/domain"
	"autoscale-distr-storage/internal/document/service"
)

type LocalExecutor struct{ service *service.Service }

func NewLocalExecutor(s *service.Service) *LocalExecutor { return &LocalExecutor{service: s} }

func (l *LocalExecutor) Execute(ctx context.Context, op string, r Request) (Response, error) {
	var record document.Record
	var err error
	switch op {
	case "create":
		record, err = l.service.Create(ctx, r.key(), document.Payload(r.Payload))
	case "get":
		record, err = l.service.Get(ctx, r.key())
	case "update":
		record, err = l.service.Update(ctx, r.key(), document.Payload(r.Payload), r.ExpectedRevision)
	case "delete":
		record, err = l.service.Delete(ctx, r.key(), r.ExpectedRevision)
	default:
		return Response{}, document.ErrInvalid
	}
	if err != nil {
		return Response{}, err
	}
	return Response{record.PartitionKey, record.ID, json.RawMessage(record.Payload), record.Revision, record.CreatedAt, record.UpdatedAt}, nil
}
