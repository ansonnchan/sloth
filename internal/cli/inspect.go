package cli

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
	"github.com/ansonnchan/sloth/internal/render"
	"github.com/ansonnchan/sloth/internal/safety"
	"github.com/spf13/cobra"
)

func newInspectCommand(stdout, stderr io.Writer, service *inspect.Service) *cobra.Command {
	options := config.Defaults()
	var headers []string
	var bodyFile string
	var colorMode = string(options.Color)
	var ascii bool
	var jsonOutput bool

	command := &cobra.Command{
		Use:   "inspect URL",
		Short: "Explain where one HTTP request spent time",
		Args: func(command *cobra.Command, args []string) error {
			return invalidUsage(cobra.ExactArgs(1)(command, args))
		},
		RunE: func(command *cobra.Command, args []string) error {
			options.Color = config.ColorMode(colorMode)
			input := config.Input{Target: args[0], Headers: headers, Options: options}
			if err := config.Validate(input); err != nil {
				return err
			}
			effective := config.Resolve(options)
			if _, err := safety.Evaluate(effective, []safety.Experiment{{Name: "inspect", Measured: 1}}); err != nil {
				return err
			}

			var body httpmeasure.BodySource
			if bodyFile != "" {
				body = fileBodySource(bodyFile)
			}
			spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
				Target:  args[0],
				Method:  effective.Method(),
				Headers: parseHeaders(headers),
				Body:    body,
				Options: httpmeasure.RequestOptions{
					Timeout:            effective.Timeout(),
					FollowRedirects:    effective.FollowRedirects(),
					RedirectLimit:      effective.RedirectLimit(),
					BodyLimit:          effective.BodyLimit(),
					AllowUnsafeMethods: effective.AllowUnsafeMethods(),
				},
			})
			if err != nil {
				return err
			}

			result, inspectErr := service.Inspect(command.Context(), spec)
			if jsonOutput {
				if err := (Output{Data: stdout, Diagnostics: stderr}).WriteJSON(inspect.NewDocument(result)); err != nil {
					return fmt.Errorf("write inspect JSON: %w", err)
				}
			}
			if inspectErr != nil {
				if result.Outcome.Error != nil {
					return result.Outcome.Error
				}
				return inspectErr
			}
			if jsonOutput {
				return nil
			}
			return render.Inspect(stdout, render.NewInspectView(result), render.Options{
				Width: terminalWidth(),
				Color: useColor(options.Color, stdout),
				ASCII: ascii,
			})
		},
	}

	flags := command.Flags()
	flags.StringVarP(&options.Method, "method", "X", options.Method, "HTTP method")
	flags.DurationVar(&options.Timeout, "timeout", options.Timeout, "request timeout")
	flags.StringArrayVarP(&headers, "header", "H", nil, "request header as 'Name: Value' (repeatable)")
	flags.StringVar(&bodyFile, "body-file", "", "read the request body from a file")
	flags.BoolVar(&options.FollowRedirects, "follow-redirects", options.FollowRedirects, "follow bounded HTTP redirects")
	flags.IntVar(&options.RedirectLimit, "redirect-limit", options.RedirectLimit, "maximum redirect hops")
	flags.Int64Var(&options.BodyLimit, "body-limit", options.BodyLimit, "maximum response-body bytes")
	flags.StringVar(&colorMode, "color", colorMode, "color mode: auto, always, or never")
	flags.BoolVar(&ascii, "ascii", false, "use ASCII-only terminal output")
	flags.BoolVar(&jsonOutput, "json", false, "write one machine-readable JSON document")
	flags.BoolVar(&options.AllowHighLoad, "allow-high-load", false, "acknowledge settings above normal safety limits")
	flags.BoolVar(&options.AllowUnsafeMethods, "allow-unsafe-methods", false, "acknowledge a method that may change server state")
	return command
}

func parseHeaders(values []string) http.Header {
	headers := make(http.Header, len(values))
	for _, value := range values {
		name, headerValue, _ := strings.Cut(value, ":")
		headers.Add(strings.TrimSpace(name), strings.TrimSpace(headerValue))
	}
	return headers
}

type fileBodySource string

func (source fileBodySource) Open() (io.ReadCloser, error) {
	file, err := os.Open(string(source))
	if err != nil {
		return nil, fmt.Errorf("open request body file: %w", err)
	}
	return file, nil
}

func terminalWidth() int {
	width, err := strconv.Atoi(os.Getenv("COLUMNS"))
	if err != nil || width < 20 {
		return 80
	}
	return width
}

func useColor(mode config.ColorMode, writer io.Writer) bool {
	switch mode {
	case config.ColorAlways:
		return true
	case config.ColorNever:
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	file, ok := writer.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
