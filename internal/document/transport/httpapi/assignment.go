package api

import "autoscale-distr-storage/internal/assignment"

// Assignment supplies the routing snapshot used by the HTTP boundary.
type Assignment interface {
	Partition(string) int
	Owner(int) assignment.Node
	Version() string
}
