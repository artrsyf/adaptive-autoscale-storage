package report

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/experiment"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteBarChart(test *testing.T) {
	chartPath := filepath.Join(test.TempDir(), "chart.svg")
	err := writeBarChart(chartPath, chartDefinition{
		filename: "chart.svg",
		title:    "Movement",
		yLabel:   "Percent",
		values: []chartValue{
			{algorithm: "modulo", transition: "3→5", value: 80},
			{algorithm: "logical_partitions", transition: "3→5", value: 40},
		},
	})
	if err != nil {
		test.Fatal(err)
	}
	chartBytes, err := os.ReadFile(chartPath)
	if err != nil {
		test.Fatal(err)
	}
	chart := string(chartBytes)
	if !strings.Contains(chart, "<svg") || !strings.Contains(chart, "logical_partitions") || !strings.Contains(chart, "80") {
		test.Fatalf("unexpected chart output: %s", chart)
	}
}

func TestChartValuesSkipInapplicablePartitions(test *testing.T) {
	partitionCount := 50
	summaries := []experiment.Summary{
		{Algorithm: "modulo", FromNodes: 3, ToNodes: 5},
		{Algorithm: "logical_partitions", FromNodes: 3, ToNodes: 5, ReassignedPartitions: &partitionCount},
	}
	values := chartValues(summaries, func(summary experiment.Summary) (float64, bool) {
		if summary.ReassignedPartitions == nil {
			return 0, false
		}
		return float64(*summary.ReassignedPartitions), true
	})
	if len(values) != 1 || values[0].algorithm != "logical_partitions" || values[0].value != 50 {
		test.Fatalf("unexpected values: %+v", values)
	}
}
