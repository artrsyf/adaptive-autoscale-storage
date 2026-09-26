package httpclient

import (
	document "autoscale-distr-storage/router/internal/document/domain"
	"autoscale-distr-storage/router/internal/document/transport/dto"
	"encoding/json"
)

// documentRequest выбирает wire DTO для уже проверенной сервисом команды.
func documentRequest(documentCommand document.Command) any {
	switch documentCommand.Operation {
	case "create":
		return dto.DocumentCreateRequest{PartitionKey: documentCommand.Key.PartitionKey, ID: documentCommand.Key.ID, Payload: json.RawMessage(documentCommand.Payload)}
	case "get":
		return dto.DocumentGetRequest{PartitionKey: documentCommand.Key.PartitionKey, ID: documentCommand.Key.ID}
	case "update":
		return dto.DocumentUpdateRequest{PartitionKey: documentCommand.Key.PartitionKey, ID: documentCommand.Key.ID, Payload: json.RawMessage(documentCommand.Payload), ExpectedRevision: documentCommand.ExpectedRevision}
	case "delete":
		return dto.DocumentDeleteRequest{PartitionKey: documentCommand.Key.PartitionKey, ID: documentCommand.Key.ID, ExpectedRevision: documentCommand.ExpectedRevision}
	default:
		return nil
	}
}

// decodeDocumentRecord читает DTO ответа выбранной операции и преобразует его в модель Router.
func decodeDocumentRecord(decoder *json.Decoder, operation string) (document.Record, error) {
	switch operation {
	case "create":
		var response dto.DocumentCreateResponse
		if err := decoder.Decode(&response); err != nil {
			return document.Record{}, err
		}
		return response.Record(), nil
	case "get":
		var response dto.DocumentGetResponse
		if err := decoder.Decode(&response); err != nil {
			return document.Record{}, err
		}
		return response.Record(), nil
	case "update":
		var response dto.DocumentUpdateResponse
		if err := decoder.Decode(&response); err != nil {
			return document.Record{}, err
		}
		return response.Record(), nil
	case "delete":
		var response dto.DocumentDeleteResponse
		if err := decoder.Decode(&response); err != nil {
			return document.Record{}, err
		}
		return response.Record(), nil
	default:
		return document.Record{}, document.ErrInvalid
	}
}
