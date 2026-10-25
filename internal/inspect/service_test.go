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

func TestServiceObservesReusedConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	defer server.Close()

	factory := func(settings ClientSettings) (ClientSession, error) {
		owner, err := httpmeasure.NewTransport(httpmeasure.TransportReuse)
		if err != nil {
			return ClientSession{}, err
		}
		client := owner.Client(settings.Timeout)
		client.CheckRedirect = settings.CheckRedirect
		response, err := client.Get(server.URL + "/warmup")
		if err != nil {
			owner.Close()
			return ClientSession{}, err
		}
		_, readErr := io.Copy(io.Discard, response.Body)
		closeErr := response.Body.Close()
		if readErr != nil || closeErr != nil {
			owner.Close()
			return ClientSession{}, errors.Join(readErr, closeErr)
		}
		return ClientSession{Client: client, Close: owner.Close}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  server.URL + "/inspect",
		Method:  http.MethodGet,
		Options: defaultRequestOptions(),
	})

	result, err := NewService(factory).Inspect(context.Background(), spec)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if !result.Connection.Observed || !result.Connection.Reused {
		t.Fatalf("connection = %#v, want observed reuse", result.Connection)
	}
	if result.Timings.Connect.Reason() != httpmeasure.UnavailableConnectionReuse || result.Timings.TLS.Reason() != httpmeasure.UnavailableConnectionReuse {
		t.Fatalf("reuse setup timings = connect %q TLS %q", result.Timings.Connect.Reason(), result.Timings.TLS.Reason())
	}
}

func TestServiceSupportsHTTP2(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "h2")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	factory := func(settings ClientSettings) (ClientSession, error) {
		client := server.Client()
		client.Timeout = settings.Timeout
		client.CheckRedirect = settings.CheckRedirect
		return ClientSession{Client: client, Close: client.CloseIdleConnections}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  server.URL,
		Method:  http.MethodGet,
		Options: defaultRequestOptions(),
	})

	result, err := NewService(factory).Inspect(context.Background(), spec)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if result.Protocol != "HTTP/2.0" || result.Outcome.Category != httpmeasure.OutcomeSuccess {
		t.Fatalf("protocol/outcome = (%q, %q)", result.Protocol, result.Outcome.Category)
	}
}

func TestServiceTimingRelationshipsForDelayedHeadersAndBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(5 * time.Millisecond)
		flusher := writer.(http.Flusher)
		_, _ = io.WriteString(writer, "first")
		flusher.Flush()
		time.Sleep(5 * time.Millisecond)
		_, _ = io.WriteString(writer, "second")
	}))
	defer server.Close()

	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target:  server.URL,
		Method:  http.MethodGet,
		Options: defaultRequestOptions(),
	})
	result, err := NewService(nil).Inspect(context.Background(), spec)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	total, totalOK := result.Timings.Total.Value()
	firstByte, firstByteOK := result.Timings.FirstByteWait.Value()
	body, bodyOK := result.Timings.BodyTransfer.Value()
	if !totalOK || !firstByteOK || !bodyOK || total < firstByte || total < body || firstByte <= 0 || body <= 0 {
		t.Fatalf("timing relationships = total (%s, %v), first byte (%s, %v), body (%s, %v)", total, totalOK, firstByte, firstByteOK, body, bodyOK)
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
