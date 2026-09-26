package utils

import (
	"fmt"
	"time"
)

// Duration поддерживает запись длительности строкой с единицами в YAML.
type Duration time.Duration

// Time возвращает длительность в формате стандартной библиотеки Go.
func (configuredDuration Duration) Time() time.Duration { return time.Duration(configuredDuration) }

// UnmarshalYAML читает длительность из YAML-строки с единицами, например 2500ms или 3s.
func (configuredDuration *Duration) UnmarshalYAML(unmarshal func(any) error) error {
	var value string
	if err := unmarshal(&value); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}
	*configuredDuration = Duration(parsed)
	return nil
}
