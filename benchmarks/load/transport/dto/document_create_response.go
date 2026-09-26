package dto

// DocumentCreateResponse читает только revision ответа create, необходимую генератору; остальные поля сервера игнорируются.
type DocumentCreateResponse struct {
	Revision string `json:"revision"`
}
