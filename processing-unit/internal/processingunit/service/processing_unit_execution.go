package service

// ProcessingUnitExecution содержит выбранные Router раздел, адресат и epoch; топологии здесь нет.
type ProcessingUnitExecution struct {
	Partition int
	NodeID    string
	Epoch     string
}
