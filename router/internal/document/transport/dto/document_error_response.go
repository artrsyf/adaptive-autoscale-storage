package dto

// DocumentErrorResponse содержит публичный код ошибки без внутренних подробностей.
type DocumentErrorResponse struct {
	Error string `json:"error"`
}
