package experiment

import "testing"

func TestLoadConfig(test *testing.T) {
	config, err := LoadConfig("../../config.yaml")
	if err != nil {
		test.Fatal(err)
	}
	if config.KeyCount != 1_000_000 || config.TimingIterations != 1000 || config.LogicalPartitions != 128 || config.VirtualNodes != 128 || len(config.Transitions) != 4 {
		test.Fatalf("unexpected supplied config: %+v", config)
	}
}

func TestRejectInvalidConfig(test *testing.T) {
	validConfig := Config{
		KeyCount:          100,
		RecordBytes:       1024,
		Repetitions:       1,
		TimingIterations:  10,
		LogicalPartitions: 128,
		VirtualNodes:      32,
		Output:            "results",
		Transitions:       []Transition{{FromNodes: 3, ToNodes: 5}},
	}
	testCases := []Config{
		{},
		func() Config { invalidConfig := validConfig; invalidConfig.KeyCount = 0; return invalidConfig }(),
		func() Config { invalidConfig := validConfig; invalidConfig.Repetitions = 0; return invalidConfig }(),
		func() Config { invalidConfig := validConfig; invalidConfig.TimingIterations = 0; return invalidConfig }(),
		func() Config {
			invalidConfig := validConfig
			invalidConfig.Transitions = []Transition{{FromNodes: 3, ToNodes: 3}}
			return invalidConfig
		}(),
		func() Config {
			invalidConfig := validConfig
			invalidConfig.Transitions = append(invalidConfig.Transitions, invalidConfig.Transitions[0])
			return invalidConfig
		}(),
	}
	for testCaseIndex, testCase := range testCases {
		if err := testCase.Validate(); err == nil {
			test.Fatalf("case %d accepted invalid config", testCaseIndex)
		}
	}
}
