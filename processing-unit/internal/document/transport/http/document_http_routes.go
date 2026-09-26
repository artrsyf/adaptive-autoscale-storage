package api

import "net/http"

// NewDocumentHttpRoutes явно регистрирует четыре POST-endpoint с единым конфигом middleware.
// Fallback сохраняет JSON-ошибки 404/405 и их учёт в метриках.
func NewDocumentHttpRoutes(documentHttpConfig DocumentHttpConfig, documentCommandService DocumentCommandService, documentHttpMetrics *DocumentHttpMetrics) *http.ServeMux {
	httpMux := http.NewServeMux()
	handler := &DocumentHttpHandler{Service: documentCommandService, Metrics: documentHttpMetrics}
	middleware := DocumentHttpMiddleware{Config: documentHttpConfig, Metrics: documentHttpMetrics}
	httpMux.Handle("POST /create", middleware.Wrap("create", handler.CreateDocument))
	httpMux.Handle("/create", middleware.Wrap("create", documentMethodNotAllowed))
	httpMux.Handle("POST /get", middleware.Wrap("get", handler.GetDocument))
	httpMux.Handle("/get", middleware.Wrap("get", documentMethodNotAllowed))
	httpMux.Handle("POST /update", middleware.Wrap("update", handler.UpdateDocument))
	httpMux.Handle("/update", middleware.Wrap("update", documentMethodNotAllowed))
	httpMux.Handle("POST /delete", middleware.Wrap("delete", handler.DeleteDocument))
	httpMux.Handle("/delete", middleware.Wrap("delete", documentMethodNotAllowed))
	httpMux.Handle("/", middleware.Wrap("unknown", documentNotFound))
	return httpMux
}

// documentMethodNotAllowed возвращает единый ответ 405 для известного пути с неверным методом.
func documentMethodNotAllowed(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	responseWriter.Header().Set("Allow", "POST")
	return documentHttpError{405, "method_not_allowed"}
}

// documentNotFound возвращает единый ответ 404 для неизвестного пути без произвольного label метрик.
func documentNotFound(responseWriter http.ResponseWriter, httpRequest *http.Request) error {
	return documentHttpError{404, "unknown_operation"}
}
