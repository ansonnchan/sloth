// Package httpmeasure defines the bounded client-side HTTP measurement model.
package httpmeasure

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const redactedValue = "[REDACTED]"

var sensitiveHeaderNames = map[string]struct{}{
	"authorization":       {},
	"proxy-authorization": {},
	"cookie":              {},
	"set-cookie":          {},
}

// BodySource opens a request body for one attempt. Implementations must return
// a new reader each time because redirects may require replaying a request.
type BodySource interface {
	Open() (io.ReadCloser, error)
}

// RequestOptions contains request behavior that is safe to persist.
type RequestOptions struct {
	Timeout            time.Duration
	FollowRedirects    bool
	RedirectLimit      int
	BodyLimit          int64
	AllowUnsafeMethods bool
}

// RequestSpecInput is the construction boundary for runtime-only request data.
type RequestSpecInput struct {
	Target            string
	Method            string
	Headers           http.Header
	Body              BodySource
	Options           RequestOptions
	SecretHeaderNames []string
}

// RequestSpec keeps raw request data private so formatting cannot accidentally
// expose credentials, query values, headers, or request bodies.
type RequestSpec struct {
	target            *url.URL
	method            string
	headers           http.Header
	body              BodySource
	options           RequestOptions
	secretHeaderNames map[string]struct{}
	identity          TargetIdentity
}

// TargetIdentity is the serialization-safe identity of an HTTP endpoint.
// Query keys are retained for compatibility checks, but their values are not.
type TargetIdentity struct {
	Scheme      string   `json:"scheme"`
	Host        string   `json:"host"`
	Port        string   `json:"port,omitempty"`
	Path        string   `json:"path"`
	QueryKeys   []string `json:"query_keys,omitempty"`
	RedactedURL string   `json:"redacted_url"`
	Fingerprint string   `json:"fingerprint"`
}

// RequestMetadata is safe to render or persist. Runtime request data is never
// embedded in this value.
type RequestMetadata struct {
	Method  string         `json:"method"`
	Target  TargetIdentity `json:"target"`
	Headers http.Header    `json:"headers,omitempty"`
	Options RequestOptions `json:"options"`
}

// NewRequestSpec constructs a runtime request specification and its safe
// identity without including raw target data in returned errors.
func NewRequestSpec(input RequestSpecInput) (RequestSpec, error) {
	target, err := url.ParseRequestURI(input.Target)
	if err != nil {
		return RequestSpec{}, errors.New("parse target URL: target is malformed")
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return RequestSpec{}, errors.New("parse target URL: scheme must be http or https")
	}
	if target.Hostname() == "" {
		return RequestSpec{}, errors.New("parse target URL: host is required")
	}
	if target.User != nil {
		return RequestSpec{}, errors.New("parse target URL: URL user information is not allowed")
	}

	identity := newTargetIdentity(target)
	secretNames := make(map[string]struct{}, len(input.SecretHeaderNames))
	for _, name := range input.SecretHeaderNames {
		if normalized := strings.ToLower(strings.TrimSpace(name)); normalized != "" {
			secretNames[normalized] = struct{}{}
		}
	}

	return RequestSpec{
		target:            cloneURL(target),
		method:            strings.ToUpper(strings.TrimSpace(input.Method)),
		headers:           input.Headers.Clone(),
		body:              input.Body,
		options:           input.Options,
		secretHeaderNames: secretNames,
		identity:          identity,
	}, nil
}

// URL returns a copy of the runtime URL for request execution.
func (s RequestSpec) URL() *url.URL { return cloneURL(s.target) }

// Method returns the normalized HTTP method.
func (s RequestSpec) Method() string { return s.method }

// Headers returns a copy of the runtime headers.
func (s RequestSpec) Headers() http.Header { return s.headers.Clone() }

// Body returns the runtime-only body source.
func (s RequestSpec) Body() BodySource { return s.body }

// Options returns the bounded request behavior.
func (s RequestSpec) Options() RequestOptions { return s.options }

// Identity returns a copy of the safe target identity.
func (s RequestSpec) Identity() TargetIdentity {
	identity := s.identity
	identity.QueryKeys = append([]string(nil), identity.QueryKeys...)
	return identity
}

// Metadata creates a rendering- and persistence-safe request description.
func (s RequestSpec) Metadata() RequestMetadata {
	return RequestMetadata{
		Method:  s.method,
		Target:  s.Identity(),
		Headers: redactHeaders(s.headers, s.secretHeaderNames),
		Options: s.options,
	}
}

// String intentionally exposes only safe request metadata.
func (s RequestSpec) String() string {
	return fmt.Sprintf("%s %s", s.method, s.identity.RedactedURL)
}

// Format keeps diagnostic formatting safe even for Go-syntax verbs such as
// %#v, which would otherwise walk runtime-only fields.
func (s RequestSpec) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, s.String())
}

func newTargetIdentity(target *url.URL) TargetIdentity {
	scheme := strings.ToLower(target.Scheme)
	host := strings.ToLower(strings.TrimSuffix(target.Hostname(), "."))
	port := target.Port()
	if (scheme == "http" && port == "80") || (scheme == "https" && port == "443") {
		port = ""
	}

	authority := host
	if strings.Contains(host, ":") {
		authority = "[" + host + "]"
	}
	if port != "" {
		authority = net.JoinHostPort(host, port)
	}

	requestPath := target.EscapedPath()
	if requestPath == "" {
		requestPath = "/"
	}
	pathValue := target.Path
	rawPath := target.RawPath
	if pathValue == "" {
		pathValue = "/"
		rawPath = ""
	}

	queryKeys := make([]string, 0)
	for key, values := range target.Query() {
		count := len(values)
		if count == 0 {
			count = 1
		}
		for range count {
			queryKeys = append(queryKeys, key)
		}
	}
	sort.Strings(queryKeys)

	redactedQuery := make(url.Values)
	for _, key := range queryKeys {
		redactedQuery[key] = append(redactedQuery[key], redactedValue)
	}
	canonical := (&url.URL{
		Scheme:   scheme,
		Host:     authority,
		Path:     pathValue,
		RawPath:  rawPath,
		RawQuery: redactedQuery.Encode(),
	}).String()
	digest := sha256.Sum256([]byte(canonical))

	return TargetIdentity{
		Scheme:      scheme,
		Host:        host,
		Port:        port,
		Path:        requestPath,
		QueryKeys:   queryKeys,
		RedactedURL: canonical,
		Fingerprint: hex.EncodeToString(digest[:]),
	}
}

func redactHeaders(headers http.Header, configured map[string]struct{}) http.Header {
	redacted := make(http.Header, len(headers))
	for name, values := range headers {
		if isSensitiveHeader(name, configured) {
			redacted[name] = []string{redactedValue}
			continue
		}
		redacted[name] = append([]string(nil), values...)
	}
	return redacted
}

func isSensitiveHeader(name string, configured map[string]struct{}) bool {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if _, ok := sensitiveHeaderNames[normalized]; ok {
		return true
	}
	if _, ok := configured[normalized]; ok {
		return true
	}
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "api-key") ||
		strings.Contains(normalized, "apikey")
}

func cloneURL(target *url.URL) *url.URL {
	if target == nil {
		return nil
	}
	clone := *target
	if target.User != nil {
		user := *target.User
		clone.User = &user
	}
	return &clone
}
