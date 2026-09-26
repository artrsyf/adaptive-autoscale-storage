package document

// Command is an application operation, independent of its HTTP representation.
type Command struct {
	Operation        string
	Key              Key
	Payload          Payload
	ExpectedRevision string
}

// Validate проверяет ключ, операцию и допустимость payload и expected_revision для выбранной команды.
func (documentCommand Command) Validate() error {
	if err := documentCommand.Key.Validate(); err != nil {
		return err
	}
	switch documentCommand.Operation {
	case "create", "update":
		if err := documentCommand.Payload.Validate(); err != nil {
			return err
		}
	case "get", "delete":
		if len(documentCommand.Payload) != 0 {
			return ErrInvalid
		}
	default:
		return ErrInvalid
	}
	if documentCommand.Operation == "update" || documentCommand.Operation == "delete" {
		return ValidateRevision(documentCommand.ExpectedRevision)
	}
	if documentCommand.ExpectedRevision != "" {
		return ErrInvalid
	}
	return nil
}
