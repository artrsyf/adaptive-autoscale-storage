package api

import (
	document "autoscale-distr-storage/processing-unit/internal/document/domain"
	"autoscale-distr-storage/processing-unit/internal/processingunit/service"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// decodeDocumentRequest читает один JSON-объект выбранного DTO, запрещая неизвестные поля и второе значение.
func decodeDocumentRequest(httpRequest *http.Request, target any) error {
	decoder := json.NewDecoder(httpRequest.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(target)
	if err == nil {
		err = decoder.Decode(new(any))
		if err == io.EOF {
			return nil
		}
	}
	var size *http.MaxBytesError
	if errors.As(err, &size) {
		return documentHttpError{413, "body_too_large"}
	}
	return documentHttpError{400, "invalid_json"}
}

// writeDocumentResponse записывает DTO конкретной операции с заданным статусом.
func writeDocumentResponse(responseWriter http.ResponseWriter, status int, response any) {
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(status)
	_ = json.NewEncoder(responseWriter).Encode(response)
}

// executeDocumentCommand вызывает прикладной порт, учитывает активные вызовы и переносит метаданные маршрута в заголовки.
// Операция уже задана конкретным DTO; выбора обработчика здесь нет.
func executeDocumentCommand(responseWriter http.ResponseWriter, httpRequest *http.Request, documentCommandService DocumentCommandService, documentHttpMetrics *DocumentHttpMetrics, documentCommand document.Command) (document.Record, error) {
	documentHttpMetrics.Active.Inc()
	defer documentHttpMetrics.Active.Dec()
	partition, err := strconv.Atoi(httpRequest.Header.Get("X-Partition-ID"))
	if err != nil {
		return document.Record{}, documentHttpError{400, "invalid_request"}
	}
	execution := service.ProcessingUnitExecution{Partition: partition, NodeID: httpRequest.Header.Get("X-PU-ID"), Epoch: httpRequest.Header.Get("X-Assignment-Epoch")}
	record, err := documentCommandService.Execute(httpRequest.Context(), documentCommand, execution)
	if err != nil {
		return document.Record{}, err
	}
	responseWriter.Header().Set("X-Partition-ID", strconv.Itoa(execution.Partition))
	responseWriter.Header().Set("X-PU-ID", execution.NodeID)
	responseWriter.Header().Set("X-Assignment-Epoch", execution.Epoch)
	return record, nil
}
