package dto

// DocumentGetRequest содержит параметры HTTP-команды get нагрузочного клиента.
type DocumentGetRequest struct {
	PartitionKey string `json:"partition_key"`
	ID           string `json:"id"`
}
