package config

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func validInput() Input {
	return Input{
		Target:  "https://api.example.com/health?verbose=true",
		Headers: []string{"Accept: application/json"},
		Options: Defaults(),
	}
}

func TestValidateAcceptsValidInput(t *testing.T) {
	tests := []struct {
		name   string
		target string
		method string
	}{
		{"http", "http://localhost:8080/health", "get"},
		{"https ipv6", "https://[::1]:8443/health", "HEAD"},
		{"escaped path", "https://example.com/a%20b", "OPTIONS"},
		{"unsafe method deferred to policy", "https://example.com", "POST"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			input.Target = test.target
			input.Options.Method = test.method
			if err := Validate(input); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestValidateRejectsTargetsWithoutLeakingSecrets(t *testing.T) {
	tests := []struct {
		name   string
		target string
	}{
		{"empty", ""},
		{"unsupported scheme", "ftp://example.com/path?token=hunter2"},
		{"missing host", "https:///path?api_key=hunter2"},
		{"userinfo", "https://alice:hunter2@example.com/path"},
		{"malformed", "https://example.com/%zz?secret=hunter2"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			input.Target = test.target
			err := Validate(input)
			if err == nil {
				t.Fatal("Validate() error = nil")
			}
			for _, secret := range []string{"hunter2", "alice", "token", "api_key", "secret"} {
				if strings.Contains(err.Error(), secret) {
					t.Fatalf("error leaked %q: %v", secret, err)
				}
			}
		})
	}
}

func TestValidateRejectsMalformedHeadersWithoutEchoingValues(t *testing.T) {
	tests := []string{
		"missing delimiter",
		": secret-value",
		"Bad Header: secret-value",
		"Authorization: secret-value\r\nInjected: true",
	}

	for _, header := range tests {
		input := validInput()
		input.Headers = []string{header}
		err := Validate(input)
		if err == nil {
			t.Fatalf("Validate(%q) error = nil", header)
		}
		if strings.Contains(err.Error(), "secret-value") {
			t.Fatalf("error leaked header value: %v", err)
		}
	}
}

func TestValidateRejectsInvalidMethods(t *testing.T) {
	for _, method := range []string{"", "CONNECT", "TRACE", "NOT A METHOD"} {
		input := validInput()
		input.Options.Method = method
		if err := Validate(input); err == nil {
			t.Fatalf("Validate() accepted method %q", method)
		}
	}
}

func TestValidateRejectsInvalidBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Options)
	}{
		{"zero timeout", func(options *Options) { options.Timeout = 0 }},
		{"negative timeout", func(options *Options) { options.Timeout = -time.Second }},
		{"zero requests", func(options *Options) { options.Requests = 0 }},
		{"negative warmups", func(options *Options) { options.Warmups = -1 }},
		{"zero rate", func(options *Options) { options.Rate = 0 }},
		{"not-a-number rate", func(options *Options) { options.Rate = math.NaN() }},
		{"infinite rate", func(options *Options) { options.Rate = math.Inf(1) }},
		{"empty concurrency", func(options *Options) { options.ConcurrencyStages = nil }},
		{"zero concurrency", func(options *Options) { options.ConcurrencyStages = []int{0} }},
		{"unordered concurrency", func(options *Options) { options.ConcurrencyStages = []int{5, 1} }},
		{"duplicate concurrency", func(options *Options) { options.ConcurrencyStages = []int{1, 1} }},
		{"zero redirect limit", func(options *Options) { options.RedirectLimit = 0 }},
		{"zero body limit", func(options *Options) { options.BodyLimit = 0 }},
		{"zero sample limit", func(options *Options) { options.SampleLimit = 0 }},
		{"invalid color", func(options *Options) { options.Color = "sometimes" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := validInput()
			test.mutate(&input.Options)
			if err := Validate(input); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestValidateRejectsConflictingRequestAndConcurrency(t *testing.T) {
	input := validInput()
	input.Options.Requests = 4
	input.Options.ConcurrencyStages = []int{1, 5}

	err := Validate(input)
	if err == nil {
		t.Fatal("Validate() error = nil")
	}
	if !strings.Contains(err.Error(), "--requests 10 --concurrency 1,5,10") {
		t.Fatalf("error lacks a valid example: %v", err)
	}
}

func TestValidationErrorIsTyped(t *testing.T) {
	input := validInput()
	input.Target = ""

	var validationErr *ValidationError
	if err := Validate(input); !errors.As(err, &validationErr) {
		t.Fatalf("error type = %T, want *ValidationError", err)
	}
	if validationErr.Field != "target" {
		t.Fatalf("field = %q, want target", validationErr.Field)
	}
}
