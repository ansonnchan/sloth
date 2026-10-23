package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	if err := execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func execute(args []string, stdout, stderr io.Writer) error {
	command := newRootCommand(stdout, stderr)
	command.SetArgs(args)
	return command.Execute()
}

func newRootCommand(stdout, stderr io.Writer) *cobra.Command {
	command := &cobra.Command{
		Use:     "sloth",
		Short:   "Sloth explains client-observable API latency",
		Version: version,
		Args:    cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			return command.Help()
		},
	}

	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate("sloth {{.Version}}\n")

	return command
}
