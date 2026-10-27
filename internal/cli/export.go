package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/report"
	"github.com/ansonnchan/sloth/internal/safety"
	"github.com/spf13/cobra"
)

func newExportCommand(stdout, stderr io.Writer, info buildinfo.Info) *cobra.Command {
	options := config.Defaults()
	var headers []string
	var bodyFile string
	var outputPath string
	var force bool
	var quiet bool

	command := &cobra.Command{
		Use:   "export URL",
		Short: "Run bounded experiments and save a validated report",
		Args: func(command *cobra.Command, args []string) error {
			return invalidUsage(cobra.ExactArgs(1)(command, args))
		},
		RunE: func(command *cobra.Command, args []string) error {
			input := config.Input{Target: args[0], Headers: headers, Options: options}
			if err := config.Validate(input); err != nil {
				return err
			}
			effective := config.Resolve(options)
			budget, err := safety.Evaluate(effective, safety.StandardExperiments(effective))
			if err != nil {
				return err
			}
			resolvedPath, err := report.ResolveOutputPath(report.PathOptions{Output: outputPath})
			if err != nil {
				return err
			}

			output := Output{Data: stdout, Diagnostics: stderr, Quiet: quiet}
			if err := output.Progressf(
				"Plan: %s; minimum schedule time %s\nConfiguration: method=%s timeout=%s rate=%.2f/s requests=%d warmups=%d concurrency=%v\n",
				budget, budget.MinimumTime, effective.Method(), effective.Timeout(), effective.Rate(),
				effective.Requests(), effective.Warmups(), effective.ConcurrencyStages(),
			); err != nil {
				return fmt.Errorf("write export plan: %w", err)
			}

			var body httpmeasure.BodySource
			if bodyFile != "" {
				body = fileBodySource(bodyFile)
			}
			spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
				Target: args[0], Method: effective.Method(), Headers: parseHeaders(headers), Body: body,
				Options: httpmeasure.RequestOptions{
					Timeout: effective.Timeout(), FollowRedirects: effective.FollowRedirects(),
					RedirectLimit: effective.RedirectLimit(), BodyLimit: effective.BodyLimit(),
					AllowUnsafeMethods: effective.AllowUnsafeMethods(),
				},
			})
			if err != nil {
				return err
			}

			var progressErr error
			service := diagnose.NewService(func(stage diagnose.Stage) {
				if progressErr == nil {
					progressErr = output.Progressf("Running %s...\n", stage)
				}
			})
			result, runErr := service.Run(command.Context(), spec, effective)
			if err := errors.Join(runErr, progressErr); err != nil {
				return err
			}

			value, err := report.Build(report.BuildInput{
				Result: result, Target: spec.Identity(), Configuration: effective.Snapshot(), Tool: info,
			})
			if err != nil {
				return fmt.Errorf("build export report: %w", err)
			}
			if err := writeExportReport(resolvedPath, value, force); err != nil {
				return err
			}
			if _, err := fmt.Fprintln(stdout, resolvedPath); err != nil {
				return fmt.Errorf("write export path: %w", err)
			}
			return nil
		},
	}

	flags := command.Flags()
	flags.StringVarP(&options.Method, "method", "X", options.Method, "HTTP method")
	flags.DurationVar(&options.Timeout, "timeout", options.Timeout, "per-request timeout")
	flags.IntVar(&options.Requests, "requests", options.Requests, "measured requests per experiment arm")
	flags.IntVar(&options.Warmups, "warmups", options.Warmups, "warm-up requests per experiment arm")
	flags.Float64Var(&options.Rate, "rate", options.Rate, "aggregate request starts per second")
	flags.IntSliceVar(&options.ConcurrencyStages, "concurrency", options.ConcurrencyStages, "increasing concurrency stages")
	flags.StringArrayVarP(&headers, "header", "H", nil, "request header as 'Name: Value' (repeatable)")
	flags.StringVar(&bodyFile, "body-file", "", "read each request body from a replayable file")
	flags.BoolVar(&options.FollowRedirects, "follow-redirects", options.FollowRedirects, "follow bounded HTTP redirects")
	flags.IntVar(&options.RedirectLimit, "redirect-limit", options.RedirectLimit, "maximum redirect hops")
	flags.Int64Var(&options.BodyLimit, "body-limit", options.BodyLimit, "maximum response-body bytes per request")
	flags.StringVarP(&outputPath, "output", "o", "", "report path (default: existing Desktop)")
	flags.BoolVarP(&force, "force", "f", false, "replace an existing report path")
	flags.BoolVar(&options.IncludeSamples, "include-samples", false, "include bounded per-request timing and outcome samples")
	flags.IntVar(&options.SampleLimit, "sample-limit", options.SampleLimit, "maximum measured samples to include")
	flags.BoolVar(&quiet, "quiet", false, "suppress plan and progress output")
	flags.BoolVar(&options.AllowHighLoad, "allow-high-load", false, "acknowledge settings above normal safety limits")
	flags.BoolVar(&options.AllowUnsafeMethods, "allow-unsafe-methods", false, "acknowledge a method that may change server state")
	return command
}

func writeExportReport(path string, value report.Report, force bool) error {
	if err := report.Validate(value); err != nil {
		return fmt.Errorf("validate export report: %w", err)
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export report: %w", err)
	}
	encoded = append(encoded, '\n')
	if int64(len(encoded)) > report.MaxReportBytes {
		return fmt.Errorf("encode export report: serialized size %d exceeds %d-byte report limit; lower --sample-limit", len(encoded), report.MaxReportBytes)
	}
	if err := report.WriteAtomic(path, encoded, force); err != nil {
		return fmt.Errorf("persist export report: %w", err)
	}
	return nil
}
