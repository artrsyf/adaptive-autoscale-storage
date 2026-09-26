package service

type PartitionRequestObserver interface {
	// RecordPartitionRequest учитывает запрос к проверенному номеру раздела.
	RecordPartitionRequest(int)
}
