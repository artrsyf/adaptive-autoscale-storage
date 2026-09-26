package utils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestYAMLStrictLoading проверяет длительности и отклонение неизвестных, повторных полей и лишних YAML-документов.
func TestYAMLStrictLoading(test *testing.T) {
	for _, testCase := range []struct {
		name, source string
		valid        bool
	}{
		{"valid", "timeout: 2500ms", true},
		{"unknown", "tiemout: 1s", false},
		{"duplicate", "timeout: 1s\ntimeout: 2s", false},
		{"duration", "timeout: tomorrow", false},
		{"documents", "timeout: 1s\n---\ntimeout: 2s", false},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			path := filepath.Join(test.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte(testCase.source), 0600); err != nil {
				test.Fatal(err)
			}
			var decoderConfig struct {
				Timeout Duration `yaml:"timeout"`
			}
			err := LoadYAML(path, &decoderConfig)
			if (err == nil) != testCase.valid {
				test.Fatalf("valid=%v, error=%v", testCase.valid, err)
			}
			if testCase.valid && decoderConfig.Timeout.Time() != 2500*time.Millisecond {
				test.Fatal("wrong duration")
			}
		})
	}
}
