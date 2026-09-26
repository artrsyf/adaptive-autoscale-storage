package report

import (
	"autoscale-distr-storage-partitioning-benchmarks/internal/experiment"
	"fmt"
	"html"
	"math"
	"os"
	"path/filepath"
	"strings"
)

type chartValue struct {
	algorithm  string
	transition string
	value      float64
}

type chartDefinition struct {
	filename string
	title    string
	yLabel   string
	values   []chartValue
}

func writeCharts(chartsDirectory string, summaries []experiment.Summary) error {
	chartDefinitions := []chartDefinition{
		{filename: "moved-keys.svg", title: "Keys changing owner", yLabel: "Moved keys, %", values: chartValues(summaries, func(summary experiment.Summary) (float64, bool) { return summary.MovedKeyRatio * 100, true })},
		{filename: "balance-max-mean.svg", title: "Maximum node load relative to mean", yLabel: "Max / mean", values: chartValues(summaries, func(summary experiment.Summary) (float64, bool) { return summary.MaxMeanLoad, true })},
		{filename: "balance-cv.svg", title: "Node load coefficient of variation", yLabel: "CV", values: chartValues(summaries, func(summary experiment.Summary) (float64, bool) { return summary.LoadCoefficientOfVariation, true })},
		{filename: "reassigned-partitions.svg", title: "Logical partitions changing owner", yLabel: "Partitions", values: chartValues(summaries, func(summary experiment.Summary) (float64, bool) {
			if summary.ReassignedPartitions == nil {
				return 0, false
			}
			return float64(*summary.ReassignedPartitions), true
		})},
		{filename: "rebalance-time.svg", title: "Placement calculation time", yLabel: "Median, microseconds", values: chartValues(summaries, func(summary experiment.Summary) (float64, bool) {
			return summary.RebalanceNanoseconds.Median / 1_000, true
		})},
	}
	for _, definition := range chartDefinitions {
		if err := writeBarChart(filepath.Join(chartsDirectory, definition.filename), definition); err != nil {
			return err
		}
	}
	return nil
}

func chartValues(summaries []experiment.Summary, value func(experiment.Summary) (float64, bool)) []chartValue {
	transitionOrder := make([]string, 0)
	algorithmOrder := make([]string, 0)
	seenTransitions := make(map[string]struct{})
	seenAlgorithms := make(map[string]struct{})
	valuesByGroup := make(map[string]chartValue, len(summaries))
	for _, summary := range summaries {
		transition := fmt.Sprintf("%d→%d", summary.FromNodes, summary.ToNodes)
		if _, exists := seenTransitions[transition]; !exists {
			seenTransitions[transition] = struct{}{}
			transitionOrder = append(transitionOrder, transition)
		}
		if _, exists := seenAlgorithms[summary.Algorithm]; !exists {
			seenAlgorithms[summary.Algorithm] = struct{}{}
			algorithmOrder = append(algorithmOrder, summary.Algorithm)
		}
		metricValue, include := value(summary)
		if include {
			valuesByGroup[transition+"\x00"+summary.Algorithm] = chartValue{
				algorithm:  summary.Algorithm,
				transition: transition,
				value:      metricValue,
			}
		}
	}
	values := make([]chartValue, 0, len(valuesByGroup))
	for _, transition := range transitionOrder {
		for _, algorithm := range algorithmOrder {
			if metricValue, exists := valuesByGroup[transition+"\x00"+algorithm]; exists {
				values = append(values, metricValue)
			}
		}
	}
	return values
}

func writeBarChart(path string, definition chartDefinition) error {
	const width = 1400.0
	const height = 720.0
	const left = 105.0
	const right = 35.0
	const top = 95.0
	const bottom = 95.0
	plotWidth := width - left - right
	plotHeight := height - top - bottom
	maximumValue := 0.0
	for _, value := range definition.values {
		maximumValue = math.Max(maximumValue, value.value)
	}
	if maximumValue == 0 {
		maximumValue = 1
	}
	yMaximum := maximumValue * 1.12
	barSlotWidth := plotWidth / float64(len(definition.values))
	barWidth := math.Min(64, barSlotWidth*0.68)

	var svg strings.Builder
	fmt.Fprintf(&svg, "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 %.0f %.0f\" role=\"img\" aria-label=\"%s\">\n", width, height, html.EscapeString(definition.title))
	fmt.Fprintln(&svg, "<rect width=\"100%\" height=\"100%\" fill=\"#ffffff\"/>")
	fmt.Fprintf(&svg, "<text x=\"%.0f\" y=\"38\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"24\" font-weight=\"600\">%s</text>\n", width/2, html.EscapeString(definition.title))
	legendAlgorithms := uniqueAlgorithms(definition.values)
	legendWidth := 0.0
	for _, algorithm := range legendAlgorithms {
		legendWidth += 32 + float64(len(algorithm))*8
	}
	legendX := width - right - legendWidth
	for _, algorithm := range legendAlgorithms {
		fmt.Fprintf(&svg, "<rect x=\"%.1f\" y=\"58\" width=\"16\" height=\"16\" rx=\"2\" fill=\"%s\"/>\n", legendX, algorithmColor(algorithm))
		fmt.Fprintf(&svg, "<text x=\"%.1f\" y=\"71\" font-family=\"sans-serif\" font-size=\"13\" fill=\"#1f2933\">%s</text>\n", legendX+22, html.EscapeString(algorithm))
		legendX += 32 + float64(len(algorithm))*8
	}
	for tickIndex := 0; tickIndex <= 5; tickIndex++ {
		tickValue := yMaximum * float64(tickIndex) / 5
		yPosition := top + plotHeight - plotHeight*float64(tickIndex)/5
		fmt.Fprintf(&svg, "<line x1=\"%.1f\" y1=\"%.1f\" x2=\"%.1f\" y2=\"%.1f\" stroke=\"#dce2e8\" stroke-width=\"1\"/>\n", left, yPosition, width-right, yPosition)
		fmt.Fprintf(&svg, "<text x=\"%.1f\" y=\"%.1f\" text-anchor=\"end\" font-family=\"sans-serif\" font-size=\"14\" fill=\"#425466\">%.3g</text>\n", left-10, yPosition+5, tickValue)
	}
	fmt.Fprintf(&svg, "<text x=\"22\" y=\"%.1f\" transform=\"rotate(-90 22 %.1f)\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"16\">%s</text>\n", top+plotHeight/2, top+plotHeight/2, html.EscapeString(definition.yLabel))
	for valueIndex, value := range definition.values {
		xCenter := left + barSlotWidth*(float64(valueIndex)+0.5)
		barHeight := plotHeight * value.value / yMaximum
		yPosition := top + plotHeight - barHeight
		color := algorithmColor(value.algorithm)
		fmt.Fprintf(&svg, "<rect x=\"%.1f\" y=\"%.1f\" width=\"%.1f\" height=\"%.1f\" rx=\"3\" fill=\"%s\"/>\n", xCenter-barWidth/2, yPosition, barWidth, barHeight, color)
		fmt.Fprintf(&svg, "<text x=\"%.1f\" y=\"%.1f\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"12\" fill=\"#1f2933\">%.4g</text>\n", xCenter, math.Max(top+13, yPosition-6), value.value)
	}
	transitionRanges := groupTransitionRanges(definition.values)
	for _, transitionRange := range transitionRanges {
		xCenter := left + barSlotWidth*(float64(transitionRange.start+transitionRange.end+1)/2)
		fmt.Fprintf(&svg, "<text x=\"%.1f\" y=\"%.1f\" text-anchor=\"middle\" font-family=\"sans-serif\" font-size=\"15\" font-weight=\"600\" fill=\"#1f2933\">%s</text>\n", xCenter, top+plotHeight+28, html.EscapeString(transitionRange.transition))
	}
	fmt.Fprintln(&svg, "</svg>")
	return os.WriteFile(path, []byte(svg.String()), 0o644)
}

type transitionRange struct {
	transition string
	start      int
	end        int
}

func groupTransitionRanges(values []chartValue) []transitionRange {
	ranges := make([]transitionRange, 0)
	for valueIndex, value := range values {
		if len(ranges) == 0 || ranges[len(ranges)-1].transition != value.transition {
			ranges = append(ranges, transitionRange{transition: value.transition, start: valueIndex, end: valueIndex})
			continue
		}
		ranges[len(ranges)-1].end = valueIndex
	}
	return ranges
}

func uniqueAlgorithms(values []chartValue) []string {
	algorithms := make([]string, 0)
	seenAlgorithms := make(map[string]struct{})
	for _, value := range values {
		if _, exists := seenAlgorithms[value.algorithm]; exists {
			continue
		}
		seenAlgorithms[value.algorithm] = struct{}{}
		algorithms = append(algorithms, value.algorithm)
	}
	return algorithms
}

func algorithmColor(algorithm string) string {
	switch algorithm {
	case "modulo":
		return "#d95f59"
	case "consistent_hash":
		return "#4477aa"
	case "logical_partitions":
		return "#2a9d78"
	default:
		return "#7a6fac"
	}
}
