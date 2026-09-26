package experiment

import (
	"fmt"
	"go.yaml.in/yaml/v2"
	"io"
	"os"
	"strings"
)

type Transition struct {
	FromNodes int `json:"from_nodes" yaml:"from_nodes"`
	ToNodes   int `json:"to_nodes" yaml:"to_nodes"`
}

func (transition Transition) Name() string {
	return fmt.Sprintf("%d_to_%d", transition.FromNodes, transition.ToNodes)
}

type Config struct {
	KeyCount          int          `json:"key_count" yaml:"key_count"`
	RecordBytes       int          `json:"record_bytes" yaml:"record_bytes"`
	Seed              int64        `json:"seed" yaml:"seed"`
	Repetitions       int          `json:"repetitions" yaml:"repetitions"`
	TimingIterations  int          `json:"timing_iterations" yaml:"timing_iterations"`
	LogicalPartitions int          `json:"logical_partitions" yaml:"logical_partitions"`
	VirtualNodes      int          `json:"virtual_nodes" yaml:"virtual_nodes"`
	Output            string       `json:"output" yaml:"output"`
	Transitions       []Transition `json:"transitions" yaml:"transitions"`
}

func LoadConfig(path string) (Config, error) {
	var config Config
	configFile, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer configFile.Close()
	decoder := yaml.NewDecoder(configFile)
	decoder.SetStrict(true)
	if err = decoder.Decode(&config); err != nil {
		return config, err
	}
	if decoder.Decode(new(any)) != io.EOF {
		return config, fmt.Errorf("multiple configuration values")
	}
	if err = config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (config Config) Validate() error {
	if config.KeyCount < 1 || config.KeyCount > 100_000_000 {
		return fmt.Errorf("key_count must be between 1 and 100000000")
	}
	if config.RecordBytes < 1 || config.Repetitions < 1 || config.Repetitions > 100 || config.TimingIterations < 1 || config.TimingIterations > 1_000_000 {
		return fmt.Errorf("record_bytes must be positive, repetitions must be between 1 and 100, and timing_iterations must be between 1 and 1000000")
	}
	if config.LogicalPartitions < 1 || config.LogicalPartitions > 1_000_000 || config.VirtualNodes < 1 || config.VirtualNodes > 65_536 {
		return fmt.Errorf("invalid logical_partitions or virtual_nodes")
	}
	if strings.TrimSpace(config.Output) == "" || len(config.Transitions) == 0 {
		return fmt.Errorf("output and at least one transition are required")
	}
	seenTransitions := make(map[string]struct{}, len(config.Transitions))
	for _, transition := range config.Transitions {
		if transition.FromNodes < 1 || transition.ToNodes < 1 || transition.FromNodes > 1024 || transition.ToNodes > 1024 || transition.FromNodes == transition.ToNodes {
			return fmt.Errorf("invalid transition %d to %d", transition.FromNodes, transition.ToNodes)
		}
		if _, exists := seenTransitions[transition.Name()]; exists {
			return fmt.Errorf("duplicate transition %s", transition.Name())
		}
		seenTransitions[transition.Name()] = struct{}{}
	}
	return nil
}
