package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/ansonnchan/sloth/internal/config"
)

// ExitCode is Sloth's stable process-exit contract.
type ExitCode int

const (
	ExitSuccess     ExitCode = 0
	ExitRuntime     ExitCode = 1
	ExitUsage       ExitCode = 2
	ExitRegression  ExitCode = 3
	ExitInterrupted ExitCode = 130
)

// ExitError assigns a stable process status to a domain error.
type ExitError struct {
	Code ExitCode
	Err  error
}

func (e *ExitError) Error() string {
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

// WithExitCode classifies an error for the executable boundary.
func WithExitCode(code ExitCode, err error) error {
	if err == nil {
		return nil
	}
	return &ExitError{Code: code, Err: err}
}

type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// ExitCodeFor maps an error to Sloth's public process status.
func ExitCodeFor(err error) ExitCode {
	if err == nil {
		return ExitSuccess
	}
	if errors.Is(err, context.Canceled) {
		return ExitInterrupted
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}

	var validationErr *config.ValidationError
	var commandUsageErr *usageError
	if errors.As(err, &validationErr) || errors.As(err, &commandUsageErr) {
		return ExitUsage
	}
	return ExitRuntime
}

func invalidUsage(err error) error {
	if err == nil {
		return nil
	}
	return &usageError{err: fmt.Errorf("invalid command usage: %w", err)}
}
