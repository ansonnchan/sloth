// Package config defines Sloth's shared runtime configuration.
package config

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultMethod          = http.MethodGet
	DefaultTimeout         = 30 * time.Second
	DefaultRequests        = 100
	DefaultWarmups         = 5
	DefaultRate            = 5.0
	DefaultFollowRedirects = false
	DefaultColor           = ColorAuto
	DefaultBodyLimit       = int64(1 << 20)
	DefaultIncludeSamples  = false
)

// ColorMode controls whether human-readable output uses ANSI color.
type ColorMode string

const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// Options is the mutable configuration boundary used while parsing CLI flags.
// Resolve copies it into an Effective value before orchestration begins.
type Options struct {
	Method            string
	Timeout           time.Duration
	Requests          int
	Warmups           int
	Rate              float64
	ConcurrencyStages []int
	FollowRedirects   bool
	Color             ColorMode
	BodyLimit         int64
	IncludeSamples    bool
}

// Defaults returns a fresh set of command options.
func Defaults() Options {
	return Options{
		Method:            DefaultMethod,
		Timeout:           DefaultTimeout,
		Requests:          DefaultRequests,
		Warmups:           DefaultWarmups,
		Rate:              DefaultRate,
		ConcurrencyStages: []int{1, 5, 10},
		FollowRedirects:   DefaultFollowRedirects,
		Color:             DefaultColor,
		BodyLimit:         DefaultBodyLimit,
		IncludeSamples:    DefaultIncludeSamples,
	}
}

// Effective is an immutable runtime configuration. Collection-valued getters
// return copies so callers cannot change configuration after resolution.
type Effective struct {
	method            string
	timeout           time.Duration
	requests          int
	warmups           int
	rate              float64
	concurrencyStages []int
	followRedirects   bool
	color             ColorMode
	bodyLimit         int64
	includeSamples    bool
}

// Resolve snapshots parsed options for use by orchestration and reporting.
func Resolve(options Options) Effective {
	return Effective{
		method:            strings.ToUpper(strings.TrimSpace(options.Method)),
		timeout:           options.Timeout,
		requests:          options.Requests,
		warmups:           options.Warmups,
		rate:              options.Rate,
		concurrencyStages: append([]int(nil), options.ConcurrencyStages...),
		followRedirects:   options.FollowRedirects,
		color:             options.Color,
		bodyLimit:         options.BodyLimit,
		includeSamples:    options.IncludeSamples,
	}
}

func (c Effective) Method() string           { return c.method }
func (c Effective) Timeout() time.Duration   { return c.timeout }
func (c Effective) Requests() int            { return c.requests }
func (c Effective) Warmups() int             { return c.warmups }
func (c Effective) Rate() float64            { return c.rate }
func (c Effective) FollowRedirects() bool    { return c.followRedirects }
func (c Effective) Color() ColorMode         { return c.color }
func (c Effective) BodyLimit() int64         { return c.bodyLimit }
func (c Effective) IncludeSamples() bool     { return c.includeSamples }
func (c Effective) ConcurrencyStages() []int { return append([]int(nil), c.concurrencyStages...) }

// Snapshot is the secret-free, serialization-facing configuration shape.
type Snapshot struct {
	Method            string    `json:"method"`
	TimeoutMillis     int64     `json:"timeout_ms"`
	Requests          int       `json:"requests"`
	Warmups           int       `json:"warmups"`
	Rate              float64   `json:"rate_per_second"`
	ConcurrencyStages []int     `json:"concurrency_stages"`
	FollowRedirects   bool      `json:"follow_redirects"`
	Color             ColorMode `json:"color"`
	BodyLimitBytes    int64     `json:"body_limit_bytes"`
	IncludeSamples    bool      `json:"include_samples"`
}

// Snapshot returns a serialization-safe copy of the effective configuration.
func (c Effective) Snapshot() Snapshot {
	return Snapshot{
		Method:            c.method,
		TimeoutMillis:     c.timeout.Milliseconds(),
		Requests:          c.requests,
		Warmups:           c.warmups,
		Rate:              c.rate,
		ConcurrencyStages: c.ConcurrencyStages(),
		FollowRedirects:   c.followRedirects,
		Color:             c.color,
		BodyLimitBytes:    c.bodyLimit,
		IncludeSamples:    c.includeSamples,
	}
}

// MarshalJSON serializes only the safe snapshot shape.
func (c Effective) MarshalJSON() ([]byte, error) {
	return json.Marshal(c.Snapshot())
}
