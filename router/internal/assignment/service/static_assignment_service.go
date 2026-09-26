// Package service computes routes from a validated static assignment.
package service

import (
	"autoscale-distr-storage/router/internal/assignment/domain"
	"crypto/sha256"
	"encoding/binary"
)

// StaticAssignmentService вычисляет раздел и владельца по неизменяемому снимку конфигурации.
type StaticAssignmentService struct{ config AssignmentConfig }

// NewStaticAssignmentService проверяет карту и копирует список нод, изолируя его от последующих изменений конфига.
func NewStaticAssignmentService(assignmentConfig AssignmentConfig) (*StaticAssignmentService, error) {
	if err := assignmentConfig.Validate(); err != nil {
		return nil, err
	}
	assignmentConfig.Nodes = append([]assignment.AssignmentNode(nil), assignmentConfig.Nodes...)
	return &StaticAssignmentService{config: assignmentConfig}, nil
}

// Partition вычисляет раздел по SHA-256 ключа: первые восемь байт unsigned big-endian modulo число разделов.
func (staticAssignmentService *StaticAssignmentService) Partition(key string) int {
	keyHash := sha256.Sum256([]byte(key))
	return int(binary.BigEndian.Uint64(keyHash[:8]) % uint64(staticAssignmentService.config.Partitions))
}

// Owner выбирает владельца по номеру раздела modulo число нод.
func (staticAssignmentService *StaticAssignmentService) Owner(partition int) assignment.AssignmentNode {
	return staticAssignmentService.config.Nodes[partition%len(staticAssignmentService.config.Nodes)]
}

// Resolve возвращает раздел, исполнителя и epoch статической карты для ключа документа.
func (staticAssignmentService *StaticAssignmentService) Resolve(key string) assignment.AssignmentRoute {
	partitionID := staticAssignmentService.Partition(key)
	return assignment.AssignmentRoute{Partition: partitionID, Node: staticAssignmentService.Owner(partitionID), Epoch: staticAssignmentService.config.Epoch}
}
