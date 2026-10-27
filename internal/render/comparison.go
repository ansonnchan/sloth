package render

import (
	"fmt"
	"io"

	"github.com/ansonnchan/sloth/internal/compare"
)

// Comparison renders compatibility evidence before any before/after metrics.
// Direction is always written as text so meaning never depends on color.
func Comparison(writer io.Writer, compatibility compare.Compatibility, deltas compare.DeltaReport, options Options) error {
	width := options.Width
	if width < 20 {
		width = 100
	}
	if _, err := fmt.Fprintf(writer, "%s\nMode: %s\n", colorLabel("Sloth comparison", options.Color), compatibility.Mode); err != nil {
		return err
	}
	if err := writeCompatibilityWarnings(writer, compatibility, width, options.Color); err != nil {
		return err
	}
	if !compatibility.DescriptiveEligible {
		_, err := fmt.Fprintln(writer, "\nConclusion: Reports are not comparable; no deltas were calculated.")
		return err
	}

	for _, experiment := range deltas.Experiments {
		if _, err := fmt.Fprintf(writer, "\n%s: %s (%s)\n", colorLabel("Experiment", options.Color), experiment.ID, experiment.Kind); err != nil {
			return err
		}
		if err := writeMetricTable(writer, experiment.Metrics, width); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer, "\nUncertainty:"); err != nil {
		return err
	}
	if err := writeWrappedBullet(writer, "A single before/after pair describes observed change; it does not establish statistical significance.", width, "  - ", "    "); err != nil {
		return err
	}
	if hasUndefinedPercentage(deltas) {
		if err := writeWrappedBullet(writer, "N/A percentages have a zero or unavailable baseline; absolute changes remain available when both values exist.", width, "  - ", "    "); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(writer, "\nConclusion: "+comparisonConclusion(deltas))
	return err
}

func writeCompatibilityWarnings(writer io.Writer, compatibility compare.Compatibility, width int, color bool) error {
	issues := append([]compare.Mismatch(nil), compatibility.Fatal...)
	issues = append(issues, compatibility.Warnings...)
	if len(issues) == 0 {
		_, err := fmt.Fprintln(writer, "Compatibility: exact for evaluated fields")
		return err
	}
	if _, err := fmt.Fprintln(writer, colorLabel("Compatibility warnings", color)+":"); err != nil {
		return err
	}
	for _, issue := range issues {
		field := issue.Field
		if issue.ExperimentID != "" {
			field += " [" + issue.ExperimentID + "]"
		}
		message := fmt.Sprintf("%s: %s (before=%s, after=%s)", field, issue.Message, issue.Before, issue.After)
		if err := writeWrappedBullet(writer, message, width, "  - ", "    "); err != nil {
			return err
		}
	}
	return nil
}

func writeMetricTable(writer io.Writer, metrics []compare.MetricDelta, width int) error {
	if width < 104 {
		for _, metric := range metrics {
			if _, err := fmt.Fprintf(writer, "  %s\n    before=%s  after=%s\n    change=%s  percent=%s  result=%s\n",
				metric.Name, formatMetricValue(metric.Before, metric.Unit), formatMetricValue(metric.After, metric.Unit),
				formatMetricChange(metric.Absolute, metric.Unit), formatPercent(metric.PercentChange), metricResult(metric)); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(writer, "  Metric                              Before        After       Change     Percent  Result"); err != nil {
		return err
	}
	for _, metric := range metrics {
		if _, err := fmt.Fprintf(writer, "  %-34.34s %12s %12s %12s %11s  %s\n",
			metric.Name, formatMetricValue(metric.Before, metric.Unit), formatMetricValue(metric.After, metric.Unit),
			formatMetricChange(metric.Absolute, metric.Unit), formatPercent(metric.PercentChange), metricResult(metric)); err != nil {
			return err
		}
	}
	return nil
}

func formatMetricValue(value *float64, unit string) string {
	if value == nil {
		return "N/A"
	}
	return formatNumber(*value, unit, false)
}

func formatMetricChange(value *float64, unit string) string {
	if value == nil {
		return "N/A"
	}
	return formatNumber(*value, unit, true)
}

func formatNumber(value float64, unit string, signed bool) string {
	format := "%.3g"
	if signed && value > 0 {
		format = "+%.3g"
	}
	number := fmt.Sprintf(format, value)
	switch unit {
	case "%":
		return number + "%"
	case "ms":
		return number + " ms"
	case "requests/s":
		return number + "/s"
	default:
		return number
	}
}

func formatPercent(value *float64) string {
	if value == nil {
		return "N/A"
	}
	if *value > 0 {
		return fmt.Sprintf("+%.2f%%", *value)
	}
	return fmt.Sprintf("%.2f%%", *value)
}

func metricResult(metric compare.MetricDelta) string {
	if metric.Absolute == nil {
		return "N/A"
	}
	if *metric.Absolute == 0 {
		return "unchanged"
	}
	switch metric.Direction {
	case compare.LowerIsBetter:
		if *metric.Absolute < 0 {
			return "improved"
		}
		return "regressed"
	case compare.HigherIsBetter:
		if *metric.Absolute > 0 {
			return "improved"
		}
		return "regressed"
	default:
		return "neutral"
	}
}

func comparisonConclusion(deltas compare.DeltaReport) string {
	improvements, regressions := 0, 0
	for _, experiment := range deltas.Experiments {
		for _, metric := range experiment.Metrics {
			switch metricResult(metric) {
			case "improved":
				improvements++
			case "regressed":
				regressions++
			}
		}
	}
	switch {
	case improvements > 0 && regressions > 0:
		return "Mixed observed changes; no single overall direction is claimed."
	case regressions > 0:
		return "Observed regressions in comparable metrics."
	case improvements > 0:
		return "Observed improvements in comparable metrics."
	default:
		return "No directional change was observed in available comparable metrics."
	}
}

func hasUndefinedPercentage(deltas compare.DeltaReport) bool {
	for _, experiment := range deltas.Experiments {
		for _, metric := range experiment.Metrics {
			if metric.Absolute != nil && metric.PercentChange == nil {
				return true
			}
		}
	}
	return false
}
