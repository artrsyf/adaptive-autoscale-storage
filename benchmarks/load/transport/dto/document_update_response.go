package dto

// DocumentUpdateResponse читает только revision ответа update, необходимую генератору; остальные поля сервера игнорируются.
type DocumentUpdateResponse struct {
	Revision string `json:"revision"`
}
