package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/spf13/cobra"
)

func TestProcessStreamsAndExitCodes(t *testing.T) {
	tests := []struct {
		mode       string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"help", 0, "Sloth explains client-observable API latency", ""},
		{"version", 0, "sloth dev\n", ""},
		{"usage", 2, "", "invalid command usage: unknown command \"unexpected\" for \"sloth\"\n"},
		{"runtime", 1, "", "runtime failure\n"},
		{"regression", 3, "", "regression found\n"},
		{"interrupted", 130, "", "context canceled\n"},
	}

	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=TestCLIHelperProcess", "--", test.mode)
			command.Env = append(os.Environ(), "SLOTH_CLI_HELPER=1")
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr

			err := command.Run()
			code := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("run helper: %v", err)
				}
				code = exitErr.ExitCode()
			}

			if code != test.wantCode {
				t.Fatalf("exit code = %d, want %d; stderr = %q", code, test.wantCode, stderr.String())
			}
			if test.wantStdout != "" && !strings.Contains(stdout.String(), test.wantStdout) {
				t.Fatalf("stdout = %q, want content %q", stdout.String(), test.wantStdout)
			}
			if test.wantStdout == "" && stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if test.wantStderr != "" && stderr.String() != test.wantStderr {
				t.Fatalf("stderr = %q, want %q", stderr.String(), test.wantStderr)
			}
			if test.wantStderr == "" && stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			if strings.Contains(stderr.String(), "Usage:") {
				t.Fatalf("stderr contains duplicate usage: %q", stderr.String())
			}
		})
	}
}

func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("SLOTH_CLI_HELPER") != "1" {
		return
	}

	mode := os.Args[len(os.Args)-1]
	var code ExitCode
	switch mode {
	case "help":
		code = ExitCode(Run([]string{"--help"}, os.Stdout, os.Stderr, buildinfo.Info{Version: "dev"}))
	case "version":
		code = ExitCode(Run([]string{"--version"}, os.Stdout, os.Stderr, buildinfo.Info{Version: "dev"}))
	case "usage":
		code = ExitCode(Run([]string{"unexpected"}, os.Stdout, os.Stderr, buildinfo.Info{Version: "dev"}))
	default:
		command := newRootCommand(os.Stdout, os.Stderr, buildinfo.Info{Version: "dev"})
		var err error
		switch mode {
		case "runtime":
			err = errors.New("runtime failure")
		case "regression":
			err = WithExitCode(ExitRegression, errors.New("regression found"))
		case "interrupted":
			err = context.Canceled
		}
		command.RunE = func(_ *cobra.Command, _ []string) error { return err }
		code = execute(command, []string{}, os.Stderr)
	}
	os.Exit(int(code))
}
