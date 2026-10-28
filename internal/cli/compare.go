package cli

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	comparemodel "github.com/ansonnchan/sloth/internal/compare"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/render"
	"github.com/ansonnchan/sloth/internal/report"
	"github.com/spf13/cobra"
)

func newCompareCommand(stdout, stderr io.Writer, presentation *presentationOptions) *cobra.Command {
	var threshold float64
	var metric = string(comparemodel.GateP95)

	command := &cobra.Command{
		Use:   "compare BEFORE.json AFTER.json",
		Short: "Compare compatible saved reports",
		Args: func(command *cobra.Command, args []string) error {
			return invalidUsage(cobra.ExactArgs(2)(command, args))
		},
		RunE: func(command *cobra.Command, args []string) error {
			gateMetric := comparemodel.GateMetric(strings.ToLower(metric))
			if !gateMetric.Valid() {
				return invalidUsage(fmt.Errorf("--metric must be p50, p95, or p99"))
			}
			gateEnabled := command.Flags().Changed("fail-on-regression")
			if gateEnabled && (threshold < 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0)) {
				return invalidUsage(fmt.Errorf("--fail-on-regression must be a finite non-negative percentage"))
			}
			color := presentation.colorMode()
			if color != config.ColorAuto && color != config.ColorAlways && color != config.ColorNever {
				return invalidUsage(fmt.Errorf("--color must be auto, always, or never"))
			}

			before, err := readComparisonReport("before", args[0])
			if err != nil {
				return err
			}
			after, err := readComparisonReport("after", args[1])
			if err != nil {
				return err
			}
			if before.Run.Interrupted || after.Run.Interrupted {
				return WithExitCode(ExitInterrupted, errors.New("comparison report records an interrupted run"))
			}

			mode := comparemodel.ModeDescriptive
			if gateEnabled {
				mode = comparemodel.ModeGate
			}
			compatibility := comparemodel.Check(before, after, mode)
			if gateEnabled && !compatibility.GateEligible {
				display := compatibility
				display.DescriptiveEligible = false
				if err := render.Comparison(stdout, display, comparemodel.DeltaReport{}, render.Options{Width: terminalWidth(), Color: useColor(color, stdout), ASCII: presentation.ASCII}); err != nil {
					return fmt.Errorf("write comparison: %w", err)
				}
				return WithExitCode(ExitUsage, errors.New("reports are incompatible with regression-gate requirements"))
			}
			if !compatibility.DescriptiveEligible {
				if err := render.Comparison(stdout, compatibility, comparemodel.DeltaReport{}, render.Options{Width: terminalWidth(), Color: useColor(color, stdout), ASCII: presentation.ASCII}); err != nil {
					return fmt.Errorf("write comparison: %w", err)
				}
				return WithExitCode(ExitUsage, errors.New("reports have no compatible experiment evidence"))
			}

			deltas, err := comparemodel.Calculate(compatibility)
			if err != nil {
				return WithExitCode(ExitUsage, err)
			}
			if err := render.Comparison(stdout, compatibility, deltas, render.Options{Width: terminalWidth(), Color: useColor(color, stdout), ASCII: presentation.ASCII}); err != nil {
				return fmt.Errorf("write comparison: %w", err)
			}
			if !gateEnabled {
				return nil
			}
			gate, err := comparemodel.EvaluateGate(deltas, gateMetric, threshold)
			if err != nil {
				return WithExitCode(ExitUsage, err)
			}
			if len(gate.Regressions) > 0 {
				return WithExitCode(ExitRegression, fmt.Errorf("regression gate failed: %s exceeded %.2f%% in %d experiment(s)", gateMetric, threshold, len(gate.Regressions)))
			}
			return nil
		},
	}

	flags := command.Flags()
	flags.Float64Var(&threshold, "fail-on-regression", 0, "exit 3 when the selected latency metric exceeds this percentage")
	flags.StringVar(&metric, "metric", metric, "gate metric: p50, p95, or p99")
	return command
}

func readComparisonReport(label, path string) (report.Report, error) {
	parsed, err := report.Read(path)
	if err != nil {
		return report.Report{}, WithExitCode(ExitUsage, fmt.Errorf("read %s report: %w", label, err))
	}
	return parsed.Report, nil
}
