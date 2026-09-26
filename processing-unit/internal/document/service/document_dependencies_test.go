package service

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestCoreDependencies проверяет отсутствие HTTP, PostgreSQL и Prometheus в импортах доменного ядра.
func TestCoreDependencies(test *testing.T) {
	allowed := map[string]bool{"bytes": true, "context": true, "encoding/json": true, "errors": true, "strings": true, "time": true, "unicode/utf8": true}
	for layer, outputDirectory := range map[string]string{"domain": filepath.Join("..", "domain"), "service": "."} {
		entries, err := os.ReadDir(outputDirectory)
		if err != nil {
			test.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(outputDirectory, entry.Name()), nil, parser.ImportsOnly)
			if err != nil {
				test.Fatal(err)
			}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if allowed[path] || layer == "service" && path == "autoscale-distr-storage/processing-unit/internal/document/domain" {
					continue
				}
				test.Errorf("%s/%s: forbidden core dependency %s", layer, entry.Name(), path)
			}
		}
	}
}
