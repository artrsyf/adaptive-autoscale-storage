package experiment

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/distribution"
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

type Sample struct {
	Algorithm                  string           `json:"algorithm"`
	FromNodes                  int              `json:"from_nodes"`
	ToNodes                    int              `json:"to_nodes"`
	Repetition                 int              `json:"repetition"`
	KeyCount                   int              `json:"key_count"`
	MovedKeys                  int              `json:"moved_keys"`
	MovedKeyRatio              float64          `json:"moved_key_ratio"`
	PotentialMigrationBytes    uint64           `json:"potential_migration_bytes"`
	ReassignedPartitions       *int             `json:"reassigned_partitions,omitempty"`
	MaxMeanLoad                float64          `json:"max_mean_load"`
	LoadCoefficientOfVariation float64          `json:"load_coefficient_of_variation"`
	InitialBuildNanoseconds    int64            `json:"initial_build_nanoseconds"`
	RebalanceNanoseconds       int64            `json:"rebalance_nanoseconds"`
	EvaluationNanoseconds      int64            `json:"evaluation_nanoseconds"`
	NodeLoads                  map[string]int64 `json:"node_loads"`
}

type Statistics struct {
	Minimum float64 `json:"minimum"`
	Mean    float64 `json:"mean"`
	Median  float64 `json:"median"`
	P95     float64 `json:"p95"`
	Maximum float64 `json:"maximum"`
}

type Summary struct {
	Algorithm                  string           `json:"algorithm"`
	FromNodes                  int              `json:"from_nodes"`
	ToNodes                    int              `json:"to_nodes"`
	KeyCount                   int              `json:"key_count"`
	MovedKeys                  int              `json:"moved_keys"`
	MovedKeyRatio              float64          `json:"moved_key_ratio"`
	PotentialMigrationBytes    uint64           `json:"potential_migration_bytes"`
	ReassignedPartitions       *int             `json:"reassigned_partitions,omitempty"`
	MaxMeanLoad                float64          `json:"max_mean_load"`
	LoadCoefficientOfVariation float64          `json:"load_coefficient_of_variation"`
	InitialBuildNanoseconds    Statistics       `json:"initial_build_nanoseconds"`
	RebalanceNanoseconds       Statistics       `json:"rebalance_nanoseconds"`
	EvaluationNanoseconds      Statistics       `json:"evaluation_nanoseconds"`
	NodeLoads                  map[string]int64 `json:"node_loads"`
}

func Run(config Config, algorithms []distribution.DistributionAlgorithm) ([]Sample, []Summary, error) {
	if err := config.Validate(); err != nil {
		return nil, nil, err
	}
	if len(algorithms) == 0 {
		return nil, nil, fmt.Errorf("at least one distribution algorithm is required")
	}
	keys := generateKeys(config.KeyCount, config.Seed)
	samples := make([]Sample, 0, len(algorithms)*len(config.Transitions)*config.Repetitions)
	for _, algorithm := range algorithms {
		for _, transition := range config.Transitions {
			for repetition := 1; repetition <= config.Repetitions; repetition++ {
				sample, err := runSample(config, algorithm, transition, repetition, keys)
				if err != nil {
					return nil, nil, fmt.Errorf("%s %s repetition %d: %w", algorithm.Name(), transition.Name(), repetition, err)
				}
				samples = append(samples, sample)
			}
		}
	}
	return samples, Summarize(samples), nil
}

func runSample(config Config, algorithm distribution.DistributionAlgorithm, transition Transition, repetition int, keys []string) (Sample, error) {
	initialNodes := buildNodeIDs(transition.FromNodes)
	targetNodes := buildNodeIDs(transition.ToNodes)
	initialPlacement, err := algorithm.Build(distribution.BuildRequest{Nodes: initialNodes})
	if err != nil {
		return Sample{}, err
	}
	targetPlacement, err := algorithm.Build(distribution.BuildRequest{Nodes: targetNodes, Previous: initialPlacement})
	if err != nil {
		return Sample{}, err
	}
	initialBuildDuration, err := measureBuild(algorithm, distribution.BuildRequest{Nodes: initialNodes}, config.TimingIterations)
	if err != nil {
		return Sample{}, err
	}
	rebalanceDuration, err := measureBuild(algorithm, distribution.BuildRequest{Nodes: targetNodes, Previous: initialPlacement}, config.TimingIterations)
	if err != nil {
		return Sample{}, err
	}

	nodeLoads := make(map[string]int64, len(targetNodes))
	targetNodeSet := make(map[distribution.NodeID]struct{}, len(targetNodes))
	for _, nodeID := range targetNodes {
		nodeLoads[string(nodeID)] = 0
		targetNodeSet[nodeID] = struct{}{}
	}
	movedKeys := 0
	evaluationStarted := time.Now()
	for _, key := range keys {
		initialOwner := initialPlacement.Owner(key)
		targetOwner := targetPlacement.Owner(key)
		if _, exists := targetNodeSet[targetOwner]; !exists {
			return Sample{}, fmt.Errorf("algorithm returned owner %q outside target topology", targetOwner)
		}
		if initialOwner != targetOwner {
			movedKeys++
		}
		nodeLoads[string(targetOwner)]++
	}
	evaluationDuration := time.Since(evaluationStarted)

	reassignedPartitions, err := countReassignedPartitions(initialPlacement, targetPlacement)
	if err != nil {
		return Sample{}, err
	}
	maxMeanLoad, coefficientOfVariation := loadBalance(nodeLoads)
	return Sample{
		Algorithm:                  algorithm.Name(),
		FromNodes:                  transition.FromNodes,
		ToNodes:                    transition.ToNodes,
		Repetition:                 repetition,
		KeyCount:                   len(keys),
		MovedKeys:                  movedKeys,
		MovedKeyRatio:              float64(movedKeys) / float64(len(keys)),
		PotentialMigrationBytes:    uint64(movedKeys) * uint64(config.RecordBytes),
		ReassignedPartitions:       reassignedPartitions,
		MaxMeanLoad:                maxMeanLoad,
		LoadCoefficientOfVariation: coefficientOfVariation,
		InitialBuildNanoseconds:    initialBuildDuration.Nanoseconds(),
		RebalanceNanoseconds:       rebalanceDuration.Nanoseconds(),
		EvaluationNanoseconds:      evaluationDuration.Nanoseconds(),
		NodeLoads:                  nodeLoads,
	}, nil
}

func measureBuild(algorithm distribution.DistributionAlgorithm, buildRequest distribution.BuildRequest, iterations int) (time.Duration, error) {
	const minimumMeasurementDuration = 50 * time.Millisecond
	startedAt := time.Now()
	var placement distribution.Placement
	var err error
	completedIterations := 0
	for {
		for iteration := 0; iteration < iterations; iteration++ {
			placement, err = algorithm.Build(buildRequest)
			if err != nil {
				return 0, err
			}
		}
		completedIterations += iterations
		elapsed := time.Since(startedAt)
		if elapsed >= minimumMeasurementDuration {
			if placement == nil {
				return 0, fmt.Errorf("algorithm returned a nil placement")
			}
			return elapsed / time.Duration(completedIterations), nil
		}
	}
}

func Summarize(samples []Sample) []Summary {
	type summaryAccumulator struct {
		first              Sample
		initialBuildValues []float64
		rebalanceValues    []float64
		evaluationValues   []float64
	}
	accumulators := make(map[string]*summaryAccumulator)
	orderedKeys := make([]string, 0)
	for _, sample := range samples {
		groupKey := fmt.Sprintf("%s/%d/%d", sample.Algorithm, sample.FromNodes, sample.ToNodes)
		accumulator, exists := accumulators[groupKey]
		if !exists {
			accumulator = &summaryAccumulator{first: sample}
			accumulators[groupKey] = accumulator
			orderedKeys = append(orderedKeys, groupKey)
		}
		accumulator.initialBuildValues = append(accumulator.initialBuildValues, float64(sample.InitialBuildNanoseconds))
		accumulator.rebalanceValues = append(accumulator.rebalanceValues, float64(sample.RebalanceNanoseconds))
		accumulator.evaluationValues = append(accumulator.evaluationValues, float64(sample.EvaluationNanoseconds))
	}

	summaries := make([]Summary, 0, len(orderedKeys))
	for _, groupKey := range orderedKeys {
		accumulator := accumulators[groupKey]
		firstSample := accumulator.first
		summaries = append(summaries, Summary{
			Algorithm:                  firstSample.Algorithm,
			FromNodes:                  firstSample.FromNodes,
			ToNodes:                    firstSample.ToNodes,
			KeyCount:                   firstSample.KeyCount,
			MovedKeys:                  firstSample.MovedKeys,
			MovedKeyRatio:              firstSample.MovedKeyRatio,
			PotentialMigrationBytes:    firstSample.PotentialMigrationBytes,
			ReassignedPartitions:       firstSample.ReassignedPartitions,
			MaxMeanLoad:                firstSample.MaxMeanLoad,
			LoadCoefficientOfVariation: firstSample.LoadCoefficientOfVariation,
			InitialBuildNanoseconds:    calculateStatistics(accumulator.initialBuildValues),
			RebalanceNanoseconds:       calculateStatistics(accumulator.rebalanceValues),
			EvaluationNanoseconds:      calculateStatistics(accumulator.evaluationValues),
			NodeLoads:                  firstSample.NodeLoads,
		})
	}
	return summaries
}

func calculateStatistics(values []float64) Statistics {
	orderedValues := append([]float64(nil), values...)
	sort.Float64s(orderedValues)
	total := 0.0
	for _, value := range orderedValues {
		total += value
	}
	median := orderedValues[len(orderedValues)/2]
	if len(orderedValues)%2 == 0 {
		median = (orderedValues[len(orderedValues)/2-1] + median) / 2
	}
	p95Index := int(math.Ceil(float64(len(orderedValues))*0.95)) - 1
	return Statistics{
		Minimum: orderedValues[0],
		Mean:    total / float64(len(orderedValues)),
		Median:  median,
		P95:     orderedValues[p95Index],
		Maximum: orderedValues[len(orderedValues)-1],
	}
}

func countReassignedPartitions(initialPlacement distribution.Placement, targetPlacement distribution.Placement) (*int, error) {
	initialAssignments := initialPlacement.Metadata().UnitAssignments
	targetAssignments := targetPlacement.Metadata().UnitAssignments
	if initialAssignments == nil && targetAssignments == nil {
		return nil, nil
	}
	if len(initialAssignments) != len(targetAssignments) {
		return nil, fmt.Errorf("logical assignment size changed from %d to %d", len(initialAssignments), len(targetAssignments))
	}
	reassignedPartitions := 0
	for assignmentIndex := range initialAssignments {
		if initialAssignments[assignmentIndex].Unit != targetAssignments[assignmentIndex].Unit {
			return nil, fmt.Errorf("logical assignment unit order changed")
		}
		if initialAssignments[assignmentIndex].Owner != targetAssignments[assignmentIndex].Owner {
			reassignedPartitions++
		}
	}
	return &reassignedPartitions, nil
}

func loadBalance(nodeLoads map[string]int64) (float64, float64) {
	totalLoad := int64(0)
	maximumLoad := int64(0)
	for _, nodeLoad := range nodeLoads {
		totalLoad += nodeLoad
		if nodeLoad > maximumLoad {
			maximumLoad = nodeLoad
		}
	}
	meanLoad := float64(totalLoad) / float64(len(nodeLoads))
	variance := 0.0
	for _, nodeLoad := range nodeLoads {
		difference := float64(nodeLoad) - meanLoad
		variance += difference * difference
	}
	variance /= float64(len(nodeLoads))
	return float64(maximumLoad) / meanLoad, math.Sqrt(variance) / meanLoad
}

func generateKeys(keyCount int, seed int64) []string {
	keys := make([]string, keyCount)
	keyPrefix := "key/" + strconv.FormatInt(seed, 10) + "/"
	for keyIndex := range keys {
		keys[keyIndex] = keyPrefix + strconv.Itoa(keyIndex)
	}
	return keys
}

func buildNodeIDs(nodeCount int) []distribution.NodeID {
	nodes := make([]distribution.NodeID, nodeCount)
	for nodeIndex := range nodes {
		nodes[nodeIndex] = distribution.NodeID("node-" + strconv.Itoa(nodeIndex+1))
	}
	return nodes
}
