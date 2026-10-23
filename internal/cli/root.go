// Package cli owns Sloth's command presentation and executable contract.
package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// Run executes the Sloth command and returns a stable process status.
func Run(args []string, stdout, stderr io.Writer, version string) int {
	command := newRootCommand(stdout, stderr, version)
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

func newRootCommand(stdout, stderr io.Writer, version string) *cobra.Command {
	command := &cobra.Command{
		Use:           "sloth",
		Short:         "Sloth explains client-observable API latency",
		Version:       version,
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
	return command
}
