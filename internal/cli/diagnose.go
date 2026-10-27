package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/safety"
	"github.com/spf13/cobra"
)

func newDiagnoseCommand(stdout, stderr io.Writer) *cobra.Command {
	options := config.Defaults()
	var headers []string
	var bodyFile string
	var quiet bool

	command := &cobra.Command{
		Use:   "diagnose URL",
		Short: "Run bounded experiments and explain latency patterns",
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

			output := Output{Data: stdout, Diagnostics: stderr, Quiet: quiet}
			if err := output.Progressf(
				"Plan: %s; minimum schedule time %s\nConfiguration: method=%s timeout=%s rate=%.2f/s requests=%d warmups=%d concurrency=%v\n",
				budget, budget.MinimumTime, effective.Method(), effective.Timeout(), effective.Rate(),
				effective.Requests(), effective.Warmups(), effective.ConcurrencyStages(),
			); err != nil {
				return fmt.Errorf("write diagnostic plan: %w", err)
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
			service := diagnose.NewService(func(stage diagnose.Stage) {
				_ = output.Progressf("Running %s...\n", stage)
			})
			result, runErr := service.Run(command.Context(), spec, effective)
			if err := writeDiagnoseSummary(stdout, result); err != nil {
				return errors.Join(runErr, fmt.Errorf("write diagnostic summary: %w", err))
			}
			return runErr
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
	flags.BoolVar(&quiet, "quiet", false, "suppress plan and progress output")
	flags.BoolVar(&options.AllowHighLoad, "allow-high-load", false, "acknowledge settings above normal safety limits")
	flags.BoolVar(&options.AllowUnsafeMethods, "allow-unsafe-methods", false, "acknowledge a method that may change server state")
	return command
}

func writeDiagnoseSummary(writer io.Writer, result diagnose.Result) error {
	if _, err := fmt.Fprintf(writer, "Diagnosis: %d finding(s), rules %s\n", len(result.Diagnosis.Findings), result.Diagnosis.RuleVersion); err != nil {
		return err
	}
	for _, finding := range result.Diagnosis.Findings {
		if _, err := fmt.Fprintf(writer, "- %s [%s confidence]: %s\n", finding.Code, finding.Confidence, finding.Observation); err != nil {
			return err
		}
	}
	return nil
}
