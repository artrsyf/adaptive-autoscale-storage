// Package topology implements immutable static assignments, not dynamic fencing.
package topology

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
)

type Node struct {
	ID  string
	URL string
}
type Assignment struct {
	Epoch      string
	Partitions int
	Nodes      []Node
}

func Parse(raw string, partitions int, epoch string) (*Assignment, error) {
	if partitions < 1 || partitions > 1024 || epoch == "" {
		return nil, fmt.Errorf("invalid partition count or epoch")
	}
	a := &Assignment{Epoch: epoch, Partitions: partitions}
	seen := map[string]bool{}
	for _, entry := range strings.Split(raw, ",") {
		p := strings.SplitN(strings.TrimSpace(entry), "=", 2)
		if len(p) != 2 || p[0] == "" || seen[p[0]] {
			return nil, fmt.Errorf("invalid or duplicate node")
		}
		u, err := url.Parse(p[1])
		if err != nil || u.Host == "" || u.Scheme != "http" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return nil, fmt.Errorf("node URL must be an HTTP origin")
		}
		seen[p[0]] = true
		a.Nodes = append(a.Nodes, Node{p[0], p[1]})
	}
	return a, nil
}

// Partition hashes UTF-8 bytes using SHA-256, first eight bytes as uint64 BE.
func (a *Assignment) Partition(key string) int {
	h := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(h[:8]) % uint64(a.Partitions))
}
func (a *Assignment) Owner(partition int) Node { return a.Nodes[partition%len(a.Nodes)] }
