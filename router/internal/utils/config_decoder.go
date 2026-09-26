// Package utils contains application-local technical helpers.
package utils

import (
	"fmt"
	"go.yaml.in/yaml/v2"
	"io"
	"os"
)

// LoadYAML читает ровно один YAML-документ; неизвестные и повторные поля отклоняются.
func LoadYAML(path string, target any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.SetStrict(true)
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return fmt.Errorf("%s must contain one YAML document", path)
	}
	return nil
}
