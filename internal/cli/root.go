// Package cli owns Sloth's command presentation and executable contract.
package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/inspect"
	"github.com/spf13/cobra"
)

// Run executes the Sloth command and returns a stable process status.
func Run(args []string, stdout, stderr io.Writer, info buildinfo.Info) int {
	return RunContext(context.Background(), args, stdout, stderr, info)
}

// RunContext executes Sloth with caller-controlled cancellation.
func RunContext(ctx context.Context, args []string, stdout, stderr io.Writer, info buildinfo.Info) int {
	command := newRootCommand(stdout, stderr, info)
	command.SetContext(ctx)
	return int(execute(command, args, stderr))
}

func execute(command *cobra.Command, args []string, stderr io.Writer) ExitCode {
	command.SetArgs(args)
	if err := command.Execute(); err != nil {
		fmt.Fprintln(stderr, err)
		return ExitCodeFor(err)
	}
	return ExitSuccess
}

func newRootCommand(stdout, stderr io.Writer, info buildinfo.Info) *cobra.Command {
	presentation := defaultPresentationOptions()
	command := &cobra.Command{
		Use:           "sloth",
		Short:         "Sloth explains client-observable API latency",
		Version:       info.Display(),
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(command *cobra.Command, args []string) error {
			return invalidUsage(cobra.NoArgs(command, args))
		},
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate("sloth {{.Version}}\n")
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return invalidUsage(err)
	})
	command.PersistentFlags().StringVar(&presentation.Color, "color", presentation.Color, "color mode: auto, always, or never")
	command.PersistentFlags().BoolVar(&presentation.NoColor, "no-color", false, "disable ANSI color output")
	command.PersistentFlags().BoolVar(&presentation.ASCII, "ascii", false, "use ASCII-only terminal output")
	command.AddCommand(newInspectCommand(stdout, stderr, inspect.NewService(nil), &presentation))
	command.AddCommand(newDiagnoseCommand(stdout, stderr, &presentation))
	command.AddCommand(newExportCommand(stdout, stderr, info))
	command.AddCommand(newCompareCommand(stdout, stderr, &presentation))
	return command
}
