package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/render"
	"github.com/ansonnchan/sloth/internal/safety"
	"github.com/spf13/cobra"
)

func newDiagnoseCommand(stdout, stderr io.Writer, presentation *presentationOptions) *cobra.Command {
	options := config.Defaults()
	var headers []string
	var bodyFile string
	var quiet bool
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "diagnose URL",
		Short: "Run bounded experiments and explain latency patterns",
		Args: func(command *cobra.Command, args []string) error {
			return invalidUsage(cobra.ExactArgs(1)(command, args))
		},
		RunE: func(command *cobra.Command, args []string) error {
			options.Color = presentation.colorMode()
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
			var progressErr error
			service := diagnose.NewService(func(stage diagnose.Stage) {
				if progressErr == nil {
					progressErr = output.Progressf("Running %s...\n", stage)
				}
			})
			result, runErr := service.Run(command.Context(), spec, effective)
			runErr = errors.Join(runErr, progressErr)
			document := diagnose.NewDocument(result, effective, budget)
			var outputErr error
			if jsonOutput {
				outputErr = output.WriteJSON(document)
			} else {
				outputErr = render.Diagnose(stdout, document, render.Options{
					Width: terminalWidth(), Color: useColor(options.Color, stdout), ASCII: presentation.ASCII,
				})
			}
			if outputErr != nil {
				return errors.Join(runErr, fmt.Errorf("write diagnostic report: %w", outputErr))
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
	flags.BoolVar(&jsonOutput, "json", false, "write one machine-readable JSON document")
	flags.BoolVar(&quiet, "quiet", false, "suppress plan and progress output")
	flags.BoolVar(&options.AllowHighLoad, "allow-high-load", false, "acknowledge settings above normal safety limits")
	flags.BoolVar(&options.AllowUnsafeMethods, "allow-unsafe-methods", false, "acknowledge a method that may change server state")
	return command
}
