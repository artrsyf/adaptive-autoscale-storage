package dto

// DocumentGetResponse читает только revision ответа get, необходимую генератору; остальные поля сервера игнорируются.
type DocumentGetResponse struct {
	Revision string `json:"revision"`
}
