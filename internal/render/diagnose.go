package render

import (
	"fmt"
	"io"
	"strings"

	"github.com/ansonnchan/sloth/internal/diagnose"
)

// Diagnose writes a static report suitable for terminals and redirected output.
// Progress is rendered separately by the CLI and never uses cursor controls.
func Diagnose(writer io.Writer, document diagnose.Document, options Options) error {
	width := options.Width
	if width < 20 {
		width = 80
	}
	if _, err := fmt.Fprintf(writer, "%s\nRules: %s\nPlan: %d measured + %d warm-up requests (%d total)\n\n",
		colorLabel("Sloth diagnosis", options.Color), document.RuleVersion,
		document.Plan.MeasuredRequests, document.Plan.WarmupRequests, document.Plan.TotalRequests); err != nil {
		return err
	}
	if err := writeExperimentSummary(writer, document.Experiments, width, options.Color); err != nil {
		return err
	}
	if err := writeFindings(writer, document, width, options); err != nil {
		return err
	}
	if len(document.Limitations) > 0 {
		if _, err := fmt.Fprintln(writer, "\n"+colorLabel("Limitations", options.Color)+":"); err != nil {
			return err
		}
		for _, limitation := range document.Limitations {
			if err := writeWrappedBullet(writer, limitation, width, "  - ", "    "); err != nil {
				return err
			}
		}
	}
	return writeNextSteps(writer, document, width, options.Color)
}

func writeExperimentSummary(writer io.Writer, experiments []diagnose.ExperimentSummary, width int, color bool) error {
	if _, err := fmt.Fprintln(writer, colorLabel("Experiment summary", color)+":"); err != nil {
		return err
	}
	if len(experiments) == 0 {
		_, err := fmt.Fprintln(writer, "  No experiment samples completed.")
		return err
	}
	if width < 96 {
		for _, experiment := range experiments {
			if _, err := fmt.Fprintf(writer,
				"  %s\n    samples=%d  p50=%s  p95=%s  p99=%s  errors=%d\n    offered=%.2f/s  achieved=%.2f/s  actual concurrency=%.2f\n",
				experiment.Name, experiment.Samples, formatMillis(experiment.P50Millis), formatMillis(experiment.P95Millis),
				formatMillis(experiment.P99Millis), experiment.Errors, experiment.OfferedPerSecond,
				experiment.AchievedPerSecond, experiment.ActualConcurrency); err != nil {
				return err
			}
		}
		return nil
	}
	if _, err := fmt.Fprintln(writer, "  Experiment          N      p50      p95      p99  Err  Offered  Achieved  Actual"); err != nil {
		return err
	}
	for _, experiment := range experiments {
		if _, err := fmt.Fprintf(writer, "  %-18.18s %4d %8s %8s %8s %4d %7.2f/s %8.2f/s %7.2f\n",
			experiment.Name, experiment.Samples, formatMillis(experiment.P50Millis), formatMillis(experiment.P95Millis),
			formatMillis(experiment.P99Millis), experiment.Errors, experiment.OfferedPerSecond,
			experiment.AchievedPerSecond, experiment.ActualConcurrency); err != nil {
			return err
		}
	}
	return nil
}

func writeFindings(writer io.Writer, document diagnose.Document, width int, options Options) error {
	if _, err := fmt.Fprintln(writer, "\n"+colorLabel("Findings", options.Color)+":"); err != nil {
		return err
	}
	if len(document.Findings) == 0 {
		_, err := fmt.Fprintln(writer, "  No diagnostic thresholds were crossed with the available evidence.")
		return err
	}
	separator := " — "
	if options.ASCII {
		separator = " - "
	}
	for _, finding := range document.Findings {
		if _, err := fmt.Fprintf(writer, "  %s%s%s\n", finding.Code, separator, finding.Title); err != nil {
			return err
		}
		if err := writeField(writer, "Observation", finding.Observation, width, options.Color, "    "); err != nil {
			return err
		}
		if err := writeField(writer, "Confidence", string(finding.Confidence)+" (evidence quality, not probability)", width, options.Color, "    "); err != nil {
			return err
		}
		if err := writeField(writer, "Severity", string(finding.Severity), width, options.Color, "    "); err != nil {
			return err
		}
		if len(finding.PossibleCauses) > 0 {
			if _, err := fmt.Fprintln(writer, "    "+colorLabel("Possible causes", options.Color)+":"); err != nil {
				return err
			}
			for _, cause := range finding.PossibleCauses {
				if err := writeWrappedBullet(writer, cause, width, "      - ", "        "); err != nil {
					return err
				}
			}
		}
		if err := writeField(writer, "Recommendation", finding.Recommendation, width, options.Color, "    "); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(writer, "    "+colorLabel("Evidence", options.Color)+":"); err != nil {
			return err
		}
		for _, metric := range finding.Evidence.Metrics {
			if _, err := fmt.Fprintf(writer, "      %s=%.3g %s\n", metric.Name, metric.Value, metric.Unit); err != nil {
				return err
			}
		}
		for _, note := range finding.Evidence.Notes {
			if err := writeWrappedBullet(writer, note, width, "      - ", "        "); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeNextSteps(writer io.Writer, document diagnose.Document, width int, color bool) error {
	steps := make([]string, 0, len(document.Findings))
	for _, finding := range document.Findings {
		if finding.Recommendation != "" {
			steps = appendUniqueString(steps, finding.Recommendation)
		}
	}
	if len(steps) == 0 {
		steps = append(steps, "Keep this workload unchanged when comparing a future run.")
	}
	if _, err := fmt.Fprintln(writer, "\n"+colorLabel("Next steps", color)+":"); err != nil {
		return err
	}
	for _, step := range steps {
		if err := writeWrappedBullet(writer, step, width, "  - ", "    "); err != nil {
			return err
		}
	}
	return nil
}

func writeWrappedBullet(writer io.Writer, value string, width int, firstPrefix, continuationPrefix string) error {
	lines := wrap(value, width-len(firstPrefix))
	if _, err := fmt.Fprintln(writer, firstPrefix+lines[0]); err != nil {
		return err
	}
	for _, line := range lines[1:] {
		if _, err := fmt.Fprintln(writer, continuationPrefix+line); err != nil {
			return err
		}
	}
	return nil
}

func formatMillis(value *float64) string {
	if value == nil {
		return "N/A"
	}
	return fmt.Sprintf("%.1fms", *value)
}

func appendUniqueString(values []string, candidate string) []string {
	for _, value := range values {
		if strings.EqualFold(value, candidate) {
			return values
		}
	}
	return append(values, candidate)
}
