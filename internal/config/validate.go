package config

import (
	"fmt"
	"math"
	"net/url"
	"strings"
	"unicode/utf8"
)

var supportedMethods = map[string]struct{}{
	"GET":     {},
	"HEAD":    {},
	"POST":    {},
	"PUT":     {},
	"PATCH":   {},
	"DELETE":  {},
	"OPTIONS": {},
}

// Input contains values that must be valid before Sloth performs I/O.
type Input struct {
	Target  string
	Headers []string
	Options Options
}

// ValidationError describes a user-correctable input problem.
type ValidationError struct {
	Field   string
	Problem string
	Hint    string
}

func (e *ValidationError) Error() string {
	if e.Hint == "" {
		return fmt.Sprintf("invalid %s: %s", e.Field, e.Problem)
	}
	return fmt.Sprintf("invalid %s: %s; %s", e.Field, e.Problem, e.Hint)
}

// Validate checks command input without performing filesystem or network work.
func Validate(input Input) error {
	if err := validateTarget(input.Target); err != nil {
		return err
	}
	if err := validateMethod(input.Options.Method); err != nil {
		return err
	}
	for index, header := range input.Headers {
		if err := validateHeader(index, header); err != nil {
			return err
		}
	}
	if err := validateBounds(input.Options); err != nil {
		return err
	}
	return validateCombinations(input.Options)
}

func validateTarget(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return validationError("target", "a URL is required", "use an HTTP or HTTPS URL such as https://api.example.com/health")
	}

	target, err := url.ParseRequestURI(raw)
	if err != nil {
		return validationError("target", "URL is malformed", "use an HTTP or HTTPS URL such as https://api.example.com/health")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return validationError("target", "scheme must be http or https", "example: https://api.example.com/health")
	}
	if target.Hostname() == "" {
		return validationError("target", "host is required", "example: https://api.example.com/health")
	}
	if target.User != nil {
		return validationError("target", "URL user information is not allowed", "pass credentials through an explicitly supported header instead")
	}
	return nil
}

func validateMethod(method string) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	if _, ok := supportedMethods[method]; !ok {
		return validationError("method", "unsupported HTTP method", "use GET, HEAD, POST, PUT, PATCH, DELETE, or OPTIONS")
	}
	return nil
}

func validateHeader(index int, header string) error {
	name, value, found := strings.Cut(header, ":")
	if !found || !validHeaderName(name) || !validHeaderValue(value) {
		return validationError(fmt.Sprintf("header[%d]", index), "expected a valid Name: Value pair", "example: --header 'Accept: application/json'")
	}
	return nil
}

func validHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for _, character := range name {
		if !strings.ContainsRune("!#$%&'*+-.^_`|~", character) &&
			(character < '0' || character > '9') &&
			(character < 'A' || character > 'Z') &&
			(character < 'a' || character > 'z') {
			return false
		}
	}
	return true
}

func validHeaderValue(value string) bool {
	if !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character == '\r' || character == '\n' || character == 0x7f || (character < 0x20 && character != '\t') {
			return false
		}
	}
	return true
}

func validateBounds(options Options) error {
	checks := []struct {
		invalid bool
		field   string
		problem string
		hint    string
	}{
		{options.Timeout <= 0, "timeout", "must be greater than zero", "example: --timeout 30s"},
		{options.Requests <= 0, "requests", "must be greater than zero", "example: --requests 100"},
		{options.Warmups < 0, "warmups", "must not be negative", "example: --warmups 5"},
		{options.Rate <= 0 || math.IsNaN(options.Rate) || math.IsInf(options.Rate, 0), "rate", "must be a finite value greater than zero", "example: --rate 5"},
		{len(options.ConcurrencyStages) == 0, "concurrency", "at least one stage is required", "example: --concurrency 1,5,10"},
		{options.BodyLimit <= 0, "body-limit", "must be greater than zero", "example: --body-limit 1048576"},
		{options.Color != ColorAuto && options.Color != ColorAlways && options.Color != ColorNever, "color", "must be auto, always, or never", "example: --color auto"},
	}

	for _, check := range checks {
		if check.invalid {
			return validationError(check.field, check.problem, check.hint)
		}
	}

	previous := 0
	for _, stage := range options.ConcurrencyStages {
		if stage <= 0 {
			return validationError("concurrency", "stages must be greater than zero", "example: --concurrency 1,5,10")
		}
		if stage <= previous {
			return validationError("concurrency", "stages must be unique and increasing", "example: --concurrency 1,5,10")
		}
		previous = stage
	}
	return nil
}

func validateCombinations(options Options) error {
	for _, stage := range options.ConcurrencyStages {
		if stage > options.Requests {
			return validationError("concurrency", "a stage cannot exceed the measured request count", "increase --requests or use, for example, --requests 10 --concurrency 1,5,10")
		}
	}
	return nil
}

func validationError(field, problem, hint string) error {
	return &ValidationError{Field: field, Problem: problem, Hint: hint}
}
