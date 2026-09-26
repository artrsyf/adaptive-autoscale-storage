package service

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	"fmt"
	"net/url"
	"strings"
)

type AssignmentConfig struct {
	// Epoch — Версия назначения; Router и processing unit должны использовать одинаковое значение. Это не fencing.
	Epoch string `yaml:"epoch"`
	// Partitions — Число логических разделов (1–1024); должно совпадать у Router, исполнителей и генератора нагрузки.
	Partitions int `yaml:"partitions"`
	// Nodes — Упорядоченный список исполнителей; порядок влияет на выбор владельца раздела.
	Nodes []assignment.AssignmentNode `yaml:"nodes"`
}

// Validate проверяет число разделов, epoch, уникальность идентификаторов и HTTP-адреса нод.
func (assignmentConfig *AssignmentConfig) Validate() error {
	if assignmentConfig.Partitions < 1 || assignmentConfig.Partitions > 1024 || assignmentConfig.Epoch == "" || len(assignmentConfig.Nodes) == 0 {
		return fmt.Errorf("invalid partition count, epoch or empty nodes")
	}
	seen := map[string]bool{}
	for _, assignmentNode := range assignmentConfig.Nodes {
		if strings.TrimSpace(assignmentNode.ID) == "" || seen[assignmentNode.ID] {
			return fmt.Errorf("invalid or duplicate node")
		}
		parsedURL, err := url.Parse(assignmentNode.URL)
		if err != nil || parsedURL.Host == "" || parsedURL.Scheme != "http" || parsedURL.Path != "" || parsedURL.RawQuery != "" || parsedURL.Fragment != "" || parsedURL.User != nil {
			return fmt.Errorf("node URL must be an HTTP origin")
		}
		seen[assignmentNode.ID] = true
	}
	return nil
}
