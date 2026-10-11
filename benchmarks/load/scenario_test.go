package main

import (
	"math"
	"testing"
)

func TestControllerScenarioStages(t *testing.T) {
	testCases := []struct {
		fraction float64
		stage    string
		rate     float64
	}{
		{.1, "low", 15},
		{.2, "steady", 45},
		{.4, "ramp", 82.5},
		{.52, "short_burst", 250},
		{.6, "high", 120},
		{.9, "recovery", 15},
	}
	for _, testCase := range testCases {
		rate, stage := scenarioRate("controller", 100, testCase.fraction)
		if stage != testCase.stage || math.Abs(rate-testCase.rate) > 0.000001 {
			t.Fatalf("fraction=%v stage=%s rate=%v", testCase.fraction, stage, rate)
		}
	}
}

func TestControllerWarmupUsesNeutralSteadyRate(t *testing.T) {
	rate, stage := workloadRate("controller", "warmup", 120, .8)
	if rate != 54 || stage != "inactive" {
		t.Fatalf("warmup rate = %v, stage = %q; want 54, inactive", rate, stage)
	}

	measureRate, measureStage := workloadRate("controller", "measure", 120, .8)
	if measureRate != 18 || measureStage != "recovery" {
		t.Fatalf("measure rate = %v, stage = %q; want 18, recovery", measureRate, measureStage)
	}
}
