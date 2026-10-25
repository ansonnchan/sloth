package inspect

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestServiceInspectsOneRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("X-Sloth-Test") != "present" {
			t.Error("request header was not applied")
		}
		writer.WriteHeader(http.StatusTeapot)
		_, _ = io.WriteString(writer, "response")
	}))
	defer server.Close()

	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  server.URL + "/inspect?token=secret",
		Method:  http.MethodGet,
		Headers: http.Header{"X-Sloth-Test": []string{"present"}},
		Options: defaultRequestOptions(),
	})
	result, err := NewService(nil).Inspect(context.Background(), spec)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	if result.Outcome.Category != httpmeasure.OutcomeHTTPError || result.Outcome.StatusCode != http.StatusTeapot {
		t.Fatalf("outcome = %#v", result.Outcome)
	}
	if result.Body.TransferredBytes != int64(len("response")) || !result.Body.Complete {
		t.Fatalf("body = %#v", result.Body)
	}
	if !result.Connection.Observed || result.Protocol == "" {
		t.Fatalf("connection/protocol = (%#v, %q)", result.Connection, result.Protocol)
	}
	if _, ok := result.Timings.Total.Value(); !ok {
		t.Fatalf("total timing unavailable: %q", result.Timings.Total.Reason())
	}
	if strings.Contains(result.Request.Target.RedactedURL, "secret") {
		t.Fatalf("request metadata contains secret: %#v", result.Request)
	}
	if result.Redirects.FinalTarget == nil || result.Redirects.FinalTarget.Path != "/inspect" {
		t.Fatalf("redirect result = %#v", result.Redirects)
	}
}

func TestServiceClassifiesCancellation(t *testing.T) {
	factory := func(ClientSettings) (ClientSession, error) {
		return ClientSession{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
			<-request.Context().Done()
			return nil, request.Context().Err()
		})}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  "https://example.com",
		Method:  http.MethodGet,
		Options: defaultRequestOptions(),
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewService(factory).Inspect(ctx, spec)
	if !errors.Is(err, context.Canceled) || result.Outcome.Category != httpmeasure.OutcomeCancelled {
		t.Fatalf("Inspect() = (%#v, %v), want cancelled", result.Outcome, err)
	}
}

func TestServiceDoesNotRetainBodies(t *testing.T) {
	const requestSecret = "request-body-secret"
	const responseSecret = "response-body-secret"
	factory := func(ClientSettings) (ClientSession, error) {
		return ClientSession{Client: doerFunc(func(request *http.Request) (*http.Response, error) {
			requestBytes, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if string(requestBytes) != requestSecret {
				t.Fatalf("request body = %q", requestBytes)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				Body:       io.NopCloser(strings.NewReader(responseSecret)),
				Request:    request,
			}, nil
		})}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  "https://example.com",
		Method:  http.MethodPost,
		Body:    textBodySource(requestSecret),
		Options: defaultRequestOptions(),
	})

	result, err := NewService(factory).Inspect(context.Background(), spec)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("json.Marshal(result) error = %v", err)
	}
	if strings.Contains(string(encoded), requestSecret) || strings.Contains(string(encoded), responseSecret) {
		t.Fatalf("result contains body content: %#v", result)
	}
	if result.Body.TransferredBytes != int64(len(responseSecret)) {
		t.Fatalf("body bytes = %d", result.Body.TransferredBytes)
	}
}

func defaultRequestOptions() httpmeasure.RequestOptions {
	return httpmeasure.RequestOptions{
		Timeout:       time.Second,
		RedirectLimit: 5,
		BodyLimit:     1024,
	}
}

func mustSpec(t *testing.T, input httpmeasure.RequestSpecInput) httpmeasure.RequestSpec {
	t.Helper()
	spec, err := httpmeasure.NewRequestSpec(input)
	if err != nil {
		t.Fatalf("NewRequestSpec() error = %v", err)
	}
	return spec
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) { return f(request) }

type textBodySource string

func (s textBodySource) Open() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(string(s))), nil
}
