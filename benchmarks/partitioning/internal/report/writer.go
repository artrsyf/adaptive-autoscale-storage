// Package report persists immutable experiment artifacts and static visualizations.
package report

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/experiment"
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"go.yaml.in/yaml/v2"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Environment struct {
	CreatedAt    time.Time `json:"created_at"`
	GoVersion    string    `json:"go_version"`
	OperatingOS  string    `json:"operating_system"`
	Architecture string    `json:"architecture"`
	VCSRevision  string    `json:"vcs_revision,omitempty"`
	VCSModified  *bool     `json:"vcs_modified,omitempty"`
}

func Write(config experiment.Config, samples []experiment.Sample, summaries []experiment.Summary) (string, error) {
	createdAt := time.Now().UTC()
	runDirectory := filepath.Join(config.Output, createdAt.Format("20060102T150405.000000000Z"))
	chartsDirectory := filepath.Join(runDirectory, "charts")
	if err := os.MkdirAll(chartsDirectory, 0o755); err != nil {
		return "", err
	}
	configBytes, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(runDirectory, "config.yaml"), configBytes, 0o644); err != nil {
		return "", err
	}
	if err = writeJSON(filepath.Join(runDirectory, "environment.json"), buildEnvironment(createdAt)); err != nil {
		return "", err
	}
	if err = writeJSONLines(filepath.Join(runDirectory, "samples.jsonl"), samples); err != nil {
		return "", err
	}
	if err = writeJSON(filepath.Join(runDirectory, "summary.json"), summaries); err != nil {
		return "", err
	}
	if err = writeComparisonCSV(filepath.Join(runDirectory, "comparison.csv"), summaries); err != nil {
		return "", err
	}
	if err = writeNodeLoadCSV(filepath.Join(runDirectory, "node-load.csv"), summaries); err != nil {
		return "", err
	}
	if err = writeCharts(chartsDirectory, summaries); err != nil {
		return "", err
	}
	if err = writeMarkdown(filepath.Join(runDirectory, "report.md"), config, summaries); err != nil {
		return "", err
	}
	return runDirectory, nil
}

func buildEnvironment(createdAt time.Time) Environment {
	environment := Environment{
		CreatedAt:    createdAt,
		GoVersion:    runtime.Version(),
		OperatingOS:  runtime.GOOS,
		Architecture: runtime.GOARCH,
	}
	if buildInfo, available := debug.ReadBuildInfo(); available {
		for _, setting := range buildInfo.Settings {
			switch setting.Key {
			case "vcs.revision":
				environment.VCSRevision = setting.Value
			case "vcs.modified":
				modified := setting.Value == "true"
				environment.VCSModified = &modified
			}
		}
	}
	return environment
}

func writeJSON(path string, value any) error {
	encodedValue, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	encodedValue = append(encodedValue, '\n')
	return os.WriteFile(path, encodedValue, 0o644)
}

func writeJSONLines(path string, samples []experiment.Sample) error {
	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(outputFile)
	for _, sample := range samples {
		if err = encoder.Encode(sample); err != nil {
			outputFile.Close()
			return err
		}
	}
	return outputFile.Close()
}

func writeComparisonCSV(path string, summaries []experiment.Summary) error {
	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	csvWriter := csv.NewWriter(outputFile)
	header := []string{"algorithm", "from_nodes", "to_nodes", "key_count", "moved_keys", "moved_key_ratio", "potential_migration_bytes", "reassigned_partitions", "max_mean_load", "load_cv", "initial_build_median_ns", "rebalance_median_ns", "rebalance_p95_ns", "evaluation_median_ns"}
	if err = csvWriter.Write(header); err != nil {
		outputFile.Close()
		return err
	}
	for _, summary := range summaries {
		reassignedPartitions := ""
		if summary.ReassignedPartitions != nil {
			reassignedPartitions = strconv.Itoa(*summary.ReassignedPartitions)
		}
		record := []string{
			summary.Algorithm,
			strconv.Itoa(summary.FromNodes),
			strconv.Itoa(summary.ToNodes),
			strconv.Itoa(summary.KeyCount),
			strconv.Itoa(summary.MovedKeys),
			formatFloat(summary.MovedKeyRatio),
			strconv.FormatUint(summary.PotentialMigrationBytes, 10),
			reassignedPartitions,
			formatFloat(summary.MaxMeanLoad),
			formatFloat(summary.LoadCoefficientOfVariation),
			formatFloat(summary.InitialBuildNanoseconds.Median),
			formatFloat(summary.RebalanceNanoseconds.Median),
			formatFloat(summary.RebalanceNanoseconds.P95),
			formatFloat(summary.EvaluationNanoseconds.Median),
		}
		if err = csvWriter.Write(record); err != nil {
			outputFile.Close()
			return err
		}
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

func writeNodeLoadCSV(path string, summaries []experiment.Summary) error {
	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	csvWriter := csv.NewWriter(outputFile)
	if err = csvWriter.Write([]string{"algorithm", "from_nodes", "to_nodes", "node", "keys", "share"}); err != nil {
		outputFile.Close()
		return err
	}
	for _, summary := range summaries {
		nodeIDs := make([]string, 0, len(summary.NodeLoads))
		for nodeID := range summary.NodeLoads {
			nodeIDs = append(nodeIDs, nodeID)
		}
		sort.Strings(nodeIDs)
		for _, nodeID := range nodeIDs {
			nodeLoad := summary.NodeLoads[nodeID]
			record := []string{
				summary.Algorithm,
				strconv.Itoa(summary.FromNodes),
				strconv.Itoa(summary.ToNodes),
				nodeID,
				strconv.FormatInt(nodeLoad, 10),
				formatFloat(float64(nodeLoad) / float64(summary.KeyCount)),
			}
			if err = csvWriter.Write(record); err != nil {
				outputFile.Close()
				return err
			}
		}
	}
	csvWriter.Flush()
	if err = csvWriter.Error(); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

func writeMarkdown(path string, config experiment.Config, summaries []experiment.Summary) error {
	outputFile, err := os.Create(path)
	if err != nil {
		return err
	}
	bufferedWriter := bufio.NewWriter(outputFile)
	fmt.Fprintln(bufferedWriter, "# Partitioning benchmark")
	fmt.Fprintln(bufferedWriter)
	fmt.Fprintf(bufferedWriter, "Keys: %d; modeled record size: %d bytes; repetitions: %d; timing batch: %d iterations (repeated for at least 50ms); logical partitions: %d; virtual nodes: %d.\n\n", config.KeyCount, config.RecordBytes, config.Repetitions, config.TimingIterations, config.LogicalPartitions, config.VirtualNodes)
	fmt.Fprintln(bufferedWriter, "| Algorithm | Transition | Moved keys | Migration | Reassigned partitions | Max/mean | CV | Rebalance median, µs | Rebalance p95, µs |")
	fmt.Fprintln(bufferedWriter, "|---|---:|---:|---:|---:|---:|---:|---:|---:|")
	for _, summary := range summaries {
		reassignedPartitions := "n/a"
		if summary.ReassignedPartitions != nil {
			reassignedPartitions = strconv.Itoa(*summary.ReassignedPartitions)
		}
		fmt.Fprintf(bufferedWriter, "| %s | %d → %d | %.2f%% | %.2f MiB | %s | %.4f | %.4f | %.3f | %.3f |\n",
			summary.Algorithm,
			summary.FromNodes,
			summary.ToNodes,
			summary.MovedKeyRatio*100,
			float64(summary.PotentialMigrationBytes)/(1024*1024),
			reassignedPartitions,
			summary.MaxMeanLoad,
			summary.LoadCoefficientOfVariation,
			summary.RebalanceNanoseconds.Median/1_000,
			summary.RebalanceNanoseconds.P95/1_000,
		)
	}
	fmt.Fprintln(bufferedWriter)
	fmt.Fprintln(bufferedWriter, "## Charts")
	fmt.Fprintln(bufferedWriter)
	for _, chart := range []string{"moved-keys", "balance-max-mean", "balance-cv", "reassigned-partitions", "rebalance-time"} {
		fmt.Fprintf(bufferedWriter, "![%s](charts/%s.svg)\n\n", strings.ReplaceAll(chart, "-", " "), chart)
	}
	if err = bufferedWriter.Flush(); err != nil {
		outputFile.Close()
		return err
	}
	return outputFile.Close()
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 9, 64)
}
