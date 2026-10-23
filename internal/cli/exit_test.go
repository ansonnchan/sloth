package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/ansonnchan/sloth/internal/config"
)

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want ExitCode
	}{
		{"success", nil, ExitSuccess},
		{"runtime", errors.New("failed"), ExitRuntime},
		{"usage", invalidUsage(errors.New("bad flag")), ExitUsage},
		{"validation", &config.ValidationError{Field: "target", Problem: "required"}, ExitUsage},
		{"regression", WithExitCode(ExitRegression, errors.New("regressed")), ExitRegression},
		{"interrupted", context.Canceled, ExitInterrupted},
		{"wrapped interruption", errors.Join(errors.New("run stopped"), context.Canceled), ExitInterrupted},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ExitCodeFor(test.err); got != test.want {
				t.Fatalf("ExitCodeFor() = %d, want %d", got, test.want)
			}
		})
	}
}
