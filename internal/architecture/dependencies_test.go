package architecture

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Keep the business core independent of transport, database and monitoring.
func TestCoreDependencies(t *testing.T) {
	allowed := map[string]bool{"bytes": true, "context": true, "encoding/json": true, "errors": true, "strings": true, "time": true, "unicode/utf8": true}
	for _, layer := range []string{"domain", "service"} {
		dir := filepath.Join("..", "document", layer)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, entry.Name()), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range file.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if allowed[path] || layer == "service" && path == "autoscale-distr-storage/internal/document/domain" {
					continue
				}
				t.Errorf("%s/%s: forbidden core dependency %s", layer, entry.Name(), path)
			}
		}
	}
}
