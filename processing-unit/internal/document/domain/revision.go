package document

// ValidateRevision проверяет наличие и длину непрозрачного токена версии, не сверяя его с БД.
func ValidateRevision(revision string) error {
	if revision == "" || len(revision) > 128 {
		return ErrInvalid
	}
	return nil
}
