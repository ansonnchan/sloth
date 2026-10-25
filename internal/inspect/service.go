// Package inspect orchestrates one bounded HTTP measurement.
package inspect

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"time"

	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

// HTTPDoer is the injectable HTTP execution boundary.
type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ClientSettings describes the client behavior required by one inspection.
type ClientSettings struct {
	Timeout       time.Duration
	CheckRedirect func(*http.Request, []*http.Request) error
}

// ClientSession owns an injectable client and its cleanup lifecycle.
type ClientSession struct {
	Client HTTPDoer
	Close  func()
}

// ClientFactory constructs a client session for one inspection.
type ClientFactory func(ClientSettings) (ClientSession, error)

// Service performs one inspection without owning CLI presentation.
type Service struct {
	newClient ClientFactory
}

// Result is the body-free structured result of one inspection.
type Result struct {
	Request    httpmeasure.RequestMetadata `json:"request"`
	Outcome    httpmeasure.Outcome         `json:"outcome"`
	Timings    httpmeasure.TimingPhases    `json:"timings"`
	Protocol   string                      `json:"protocol,omitempty"`
	Connection Connection                  `json:"connection"`
	Body       httpmeasure.BodyReadResult  `json:"body"`
	Redirects  httpmeasure.RedirectResult  `json:"redirects"`
}

// SchemaVersion identifies the standalone inspect JSON contract.
const SchemaVersion = "sloth.inspect/v1"

// Document is the versioned machine-readable inspect output.
type Document struct {
	SchemaVersion string `json:"schema_version"`
	Result        Result `json:"result"`
}

// NewDocument wraps a result in the current inspect schema.
func NewDocument(result Result) Document {
	return Document{SchemaVersion: SchemaVersion, Result: result}
}

// Connection contains selected-connection facts, not an inference from the
// configured transport profile.
type Connection struct {
	Observed   bool    `json:"observed"`
	Reused     bool    `json:"reused"`
	WasIdle    bool    `json:"was_idle"`
	IdleMillis float64 `json:"idle_ms"`
}

// NewService creates an inspector. A nil factory selects Sloth's owned default
// transport; tests and callers may inject a bounded client implementation.
func NewService(factory ClientFactory) *Service {
	if factory == nil {
		factory = defaultClientFactory
	}
	return &Service{newClient: factory}
}

// Inspect executes exactly one request attempt.
func (s *Service) Inspect(ctx context.Context, spec httpmeasure.RequestSpec) (Result, error) {
	result := Result{Request: spec.Metadata()}
	redirects, err := httpmeasure.NewRedirectController(spec.Options().FollowRedirects, spec.Options().RedirectLimit)
	if err != nil {
		return result, err
	}
	session, err := s.newClient(ClientSettings{
		Timeout:       spec.Options().Timeout,
		CheckRedirect: redirects.CheckRedirect,
	})
	if err != nil {
		return result, err
	}
	if session.Client == nil {
		return result, errors.New("inspect request: client factory returned no client")
	}
	if session.Close == nil {
		session.Close = func() {}
	}
	defer session.Close()

	request, err := newHTTPRequest(ctx, spec)
	if err != nil {
		return result, err
	}
	collector := httpmeasure.NewTraceCollector()
	request = request.WithContext(httptrace.WithClientTrace(request.Context(), collector.ClientTrace()))

	collector.MarkRequestStart()
	response, requestErr := session.Client.Do(request)
	statusCode := 0
	var finalURL *url.URL
	if response != nil {
		collector.MarkResponseHeaders()
		collector.ObserveProtocol(response.Proto)
		result.Protocol = response.Proto
		statusCode = response.StatusCode
		if response.Request != nil {
			finalURL = response.Request.URL
		}
	}

	terminalErr := requestErr
	if requestErr == nil && response != nil {
		result.Body, terminalErr = httpmeasure.DrainBody(response.Body, spec.Options().BodyLimit)
	} else if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if ctx.Err() != nil {
		terminalErr = ctx.Err()
	}
	collector.MarkBodyDone()

	record := collector.Snapshot()
	result.Outcome = httpmeasure.ClassifyOutcome(statusCode, result.Body.TransferredBytes, terminalErr)
	result.Timings = httpmeasure.DeriveTimingPhases(record, usesTLS(spec, finalURL))
	result.Connection = connectionFrom(record)
	result.Redirects = redirects.Result(finalURL)
	if result.Outcome.Error != nil {
		return result, result.Outcome.Error
	}
	return result, nil
}

func defaultClientFactory(settings ClientSettings) (ClientSession, error) {
	owner, err := httpmeasure.NewTransport(httpmeasure.TransportDefault)
	if err != nil {
		return ClientSession{}, err
	}
	client := owner.Client(settings.Timeout)
	client.CheckRedirect = settings.CheckRedirect
	return ClientSession{Client: client, Close: owner.Close}, nil
}

func newHTTPRequest(ctx context.Context, spec httpmeasure.RequestSpec) (*http.Request, error) {
	var body io.ReadCloser
	var err error
	if spec.Body() != nil {
		body, err = spec.Body().Open()
		if err != nil {
			return nil, err
		}
	}

	request, err := http.NewRequestWithContext(ctx, spec.Method(), spec.URL().String(), body)
	if err != nil {
		if body != nil {
			_ = body.Close()
		}
		return nil, errors.New("create HTTP request: invalid runtime specification")
	}
	request.Header = spec.Headers()
	if spec.Body() != nil {
		request.GetBody = spec.Body().Open
	}
	return request, nil
}

func connectionFrom(record httpmeasure.TraceRecord) Connection {
	if len(record.Connections) == 0 {
		return Connection{}
	}
	observed := record.Connections[len(record.Connections)-1]
	return Connection{
		Observed:   true,
		Reused:     observed.Reused,
		WasIdle:    observed.WasIdle,
		IdleMillis: float64(observed.IdleTime) / float64(time.Millisecond),
	}
}

func usesTLS(spec httpmeasure.RequestSpec, finalURL *url.URL) bool {
	if finalURL != nil {
		return finalURL.Scheme == "https"
	}
	return spec.URL().Scheme == "https"
}
