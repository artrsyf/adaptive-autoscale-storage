package experiment

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/distribution"
	"math"
	"testing"
)

func TestRunProducesComparableMetrics(test *testing.T) {
	logicalAlgorithm, err := distribution.NewLogicalPartitionAlgorithm(128)
	if err != nil {
		test.Fatal(err)
	}
	config := Config{
		KeyCount:          10_000,
		RecordBytes:       100,
		Seed:              42,
		Repetitions:       2,
		TimingIterations:  10,
		LogicalPartitions: 128,
		VirtualNodes:      32,
		Output:            "results",
		Transitions:       []Transition{{FromNodes: 3, ToNodes: 5}},
	}
	samples, summaries, err := Run(config, []distribution.DistributionAlgorithm{logicalAlgorithm})
	if err != nil {
		test.Fatal(err)
	}
	if len(samples) != 2 || len(summaries) != 1 {
		test.Fatalf("unexpected result sizes: %d samples, %d summaries", len(samples), len(summaries))
	}
	for _, sample := range samples {
		if sample.MovedKeys <= 0 || sample.MovedKeys >= config.KeyCount || sample.PotentialMigrationBytes != uint64(sample.MovedKeys*config.RecordBytes) {
			test.Fatalf("invalid movement metrics: %+v", sample)
		}
		if sample.ReassignedPartitions == nil || *sample.ReassignedPartitions != 50 {
			test.Fatalf("unexpected partition movement: %+v", sample.ReassignedPartitions)
		}
		if math.Abs(sample.MaxMeanLoad-1) > 0.1 || sample.LoadCoefficientOfVariation > 0.1 {
			test.Fatalf("unexpected imbalance: max/mean %.4f cv %.4f", sample.MaxMeanLoad, sample.LoadCoefficientOfVariation)
		}
	}
	if samples[0].MovedKeys != samples[1].MovedKeys || samples[0].NodeLoads["node-1"] != samples[1].NodeLoads["node-1"] {
		test.Fatal("deterministic metrics changed between repetitions")
	}
}

func TestModuloMovementIsHighWhenNodeCountChanges(test *testing.T) {
	config := Config{
		KeyCount:          20_000,
		RecordBytes:       1,
		Repetitions:       1,
		TimingIterations:  10,
		LogicalPartitions: 128,
		VirtualNodes:      32,
		Output:            "results",
		Transitions:       []Transition{{FromNodes: 3, ToNodes: 5}},
	}
	samples, _, err := Run(config, []distribution.DistributionAlgorithm{distribution.NewModuloAlgorithm()})
	if err != nil {
		test.Fatal(err)
	}
	if samples[0].MovedKeyRatio < 0.7 || samples[0].ReassignedPartitions != nil {
		test.Fatalf("unexpected modulo result: %+v", samples[0])
	}
}
