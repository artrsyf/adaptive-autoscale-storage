package api

import (
	"autoscale-distr-storage/router/internal/document/transport/dto"
	"net/http"
)

// DocumentHttpHandler содержит обработчики CRUD; общие ограничения задаются middleware маршрутов.
type DocumentHttpHandler struct {
	Service DocumentCommandService
	Metrics *DocumentHttpMetrics
}

// CreateDocument декодирует DTO create, вызывает сервис и возвращает DTO результата этой операции.
// Таймаут, лимит тела, ошибки и HTTP-метрики задаются общей обвязкой маршрутов.
func (documentHttpHandler *DocumentHttpHandler) CreateDocument(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	var request dto.DocumentCreateRequest
	if err := decodeDocumentRequest(httpRequest, &request); err != nil {
		return err
	}
	record, err := executeDocumentCommand(responseWriter, httpRequest, documentHttpHandler.Service, documentHttpHandler.Metrics, request.Command())
	if err != nil {
		return err
	}
	writeDocumentResponse(responseWriter, http.StatusCreated, dto.NewDocumentCreateResponse(record))
	return nil
}

// GetDocument декодирует DTO get, вызывает сервис и возвращает DTO результата этой операции.
// Таймаут, лимит тела, ошибки и HTTP-метрики задаются общей обвязкой маршрутов.
func (documentHttpHandler *DocumentHttpHandler) GetDocument(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	var request dto.DocumentGetRequest
	if err := decodeDocumentRequest(httpRequest, &request); err != nil {
		return err
	}
	record, err := executeDocumentCommand(responseWriter, httpRequest, documentHttpHandler.Service, documentHttpHandler.Metrics, request.Command())
	if err != nil {
		return err
	}
	writeDocumentResponse(responseWriter, http.StatusOK, dto.NewDocumentGetResponse(record))
	return nil
}

// UpdateDocument декодирует DTO update, вызывает сервис и возвращает DTO результата этой операции.
// Таймаут, лимит тела, ошибки и HTTP-метрики задаются общей обвязкой маршрутов.
func (documentHttpHandler *DocumentHttpHandler) UpdateDocument(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	var request dto.DocumentUpdateRequest
	if err := decodeDocumentRequest(httpRequest, &request); err != nil {
		return err
	}
	record, err := executeDocumentCommand(responseWriter, httpRequest, documentHttpHandler.Service, documentHttpHandler.Metrics, request.Command())
	if err != nil {
		return err
	}
	writeDocumentResponse(responseWriter, http.StatusOK, dto.NewDocumentUpdateResponse(record))
	return nil
}

// DeleteDocument декодирует DTO delete, вызывает сервис и возвращает DTO результата этой операции.
// Таймаут, лимит тела, ошибки и HTTP-метрики задаются общей обвязкой маршрутов.
func (documentHttpHandler *DocumentHttpHandler) DeleteDocument(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	var request dto.DocumentDeleteRequest
	if err := decodeDocumentRequest(httpRequest, &request); err != nil {
		return err
	}
	record, err := executeDocumentCommand(responseWriter, httpRequest, documentHttpHandler.Service, documentHttpHandler.Metrics, request.Command())
	if err != nil {
		return err
	}
	writeDocumentResponse(responseWriter, http.StatusOK, dto.NewDocumentDeleteResponse(record))
	return nil
}
