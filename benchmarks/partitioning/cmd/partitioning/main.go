package main

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/distribution"
	"autoscale-distr-storage-partitioning-benchmarks/internal/experiment"
	"autoscale-distr-storage-partitioning-benchmarks/internal/report"
	"flag"
	"fmt"
	"log"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to partitioning experiment YAML")
	outputOverride := flag.String("output", "", "override the result root directory")
	flag.Parse()

	config, err := experiment.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if *outputOverride != "" {
		config.Output = *outputOverride
	}
	consistentHashAlgorithm, err := distribution.NewConsistentHashAlgorithm(config.VirtualNodes)
	if err != nil {
		log.Fatal(err)
	}
	logicalPartitionAlgorithm, err := distribution.NewLogicalPartitionAlgorithm(config.LogicalPartitions)
	if err != nil {
		log.Fatal(err)
	}
	algorithms := []distribution.DistributionAlgorithm{
		distribution.NewModuloAlgorithm(),
		consistentHashAlgorithm,
		logicalPartitionAlgorithm,
	}
	samples, summaries, err := experiment.Run(config, algorithms)
	if err != nil {
		log.Fatal(err)
	}
	runDirectory, err := report.Write(config, samples, summaries)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("partitioning benchmark completed: %s\n", runDirectory)
}
