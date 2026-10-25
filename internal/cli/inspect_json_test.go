package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
)

func TestInspectJSONForHTTPStatuses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/client-error":
			writer.WriteHeader(http.StatusNotFound)
		case "/server-error":
			writer.WriteHeader(http.StatusServiceUnavailable)
		default:
			writer.WriteHeader(http.StatusOK)
		}
		_, _ = io.WriteString(writer, "body")
	}))
	defer server.Close()

	for _, test := range []struct {
		path     string
		status   int
		category string
	}{
		{path: "/success", status: 200, category: "success"},
		{path: "/client-error", status: 404, category: "http_error"},
		{path: "/server-error", status: 503, category: "http_error"},
	} {
		t.Run(test.path, func(t *testing.T) {
			code, document, stdout, stderr := runInspectJSON(t, "--color", "always", server.URL+test.path)
			if code != 0 || stderr != "" {
				t.Fatalf("Run() = %d, stderr = %q", code, stderr)
			}
			if document.SchemaVersion != "sloth.inspect/v1" || document.Result.Outcome.StatusCode != test.status || document.Result.Outcome.Category != test.category {
				t.Fatalf("document = %#v", document)
			}
			if bytes.Contains(stdout, []byte("\x1b[")) {
				t.Fatalf("JSON contains ANSI: %q", stdout)
			}
			if document.Result.Body.TransferredBytes != int64(len("body")) || !document.Result.Body.Complete {
				t.Fatalf("body = %#v", document.Result.Body)
			}
			if document.Result.Timings.Total.DurationMillis == nil || *document.Result.Timings.Total.DurationMillis < 0 {
				t.Fatalf("total timing = %#v", document.Result.Timings.Total)
			}
			if document.Result.Timings.DNS.DurationMillis != nil {
				t.Fatalf("literal-IP DNS timing unexpectedly available: %#v", document.Result.Timings.DNS)
			}
		})
	}
}

func TestInspectJSONForTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(time.Second):
			writer.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	code, document, _, stderr := runInspectJSON(t, "--timeout", "20ms", server.URL)
	if code != int(ExitRuntime) || document.Result.Outcome.Category != "timeout" {
		t.Fatalf("Run() = %d, outcome = %#v", code, document.Result.Outcome)
	}
	if stderr != "request timed out\n" || document.Result.Outcome.Complete {
		t.Fatalf("stderr/completion = (%q, %v)", stderr, document.Result.Outcome.Complete)
	}
}

func TestInspectJSONForRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/start" {
			http.Redirect(writer, request, "/final?token=redirect-secret", http.StatusFound)
			return
		}
		_, _ = io.WriteString(writer, "done")
	}))
	defer server.Close()

	code, document, stdout, stderr := runInspectJSON(t, "--follow-redirects", server.URL+"/start?token=source-secret")
	if code != 0 || stderr != "" || len(document.Result.Redirects.Hops) != 1 || !document.Result.Redirects.Hops[0].Followed {
		t.Fatalf("Run() = %d, redirects = %#v, stderr = %q", code, document.Result.Redirects, stderr)
	}
	if document.Result.Redirects.FinalTarget == nil || document.Result.Redirects.FinalTarget.Path != "/final" {
		t.Fatalf("final target = %#v", document.Result.Redirects.FinalTarget)
	}
	for _, secret := range []string{"redirect-secret", "source-secret"} {
		if bytes.Contains(stdout, []byte(secret)) {
			t.Fatalf("JSON contains redirect secret %q: %s", secret, stdout)
		}
	}
}

func TestInspectJSONForPartialBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		hijacker := writer.(http.Hijacker)
		connection, buffer, err := hijacker.Hijack()
		if err != nil {
			t.Errorf("Hijack() error = %v", err)
			return
		}
		_, _ = fmt.Fprint(buffer, "HTTP/1.1 200 OK\r\nContent-Length: 10\r\n\r\nshort")
		_ = buffer.Flush()
		_ = connection.Close()
	}))
	defer server.Close()

	code, document, _, stderr := runInspectJSON(t, server.URL)
	if code != int(ExitRuntime) || document.Result.Outcome.Category != "transport_error" || document.Result.Outcome.FailurePhase != "response_body" {
		t.Fatalf("Run() = %d, outcome = %#v", code, document.Result.Outcome)
	}
	if document.Result.Outcome.StatusCode != 200 || document.Result.Body.Complete || document.Result.Body.TransferredBytes != int64(len("short")) {
		t.Fatalf("partial evidence = outcome %#v body %#v", document.Result.Outcome, document.Result.Body)
	}
	if stderr != "response body transfer failed\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestInspectJSONDoesNotContainSecretsOrBodies(t *testing.T) {
	const (
		querySecret    = "query-secret"
		headerSecret   = "header-secret"
		responseSecret = "response-secret"
	)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, responseSecret)
	}))
	defer server.Close()

	_, _, stdout, _ := runInspectJSON(t,
		"--header", "Authorization: Bearer "+headerSecret,
		server.URL+"/?token="+querySecret,
	)
	for _, secret := range []string{querySecret, headerSecret, responseSecret} {
		if bytes.Contains(stdout, []byte(secret)) {
			t.Fatalf("JSON contains secret/body %q: %s", secret, stdout)
		}
	}
}

func runInspectJSON(t *testing.T, args ...string) (int, inspectJSONDocument, []byte, string) {
	t.Helper()
	arguments := append([]string{"inspect", "--json"}, args...)
	var stdout, stderr bytes.Buffer
	code := Run(arguments, &stdout, &stderr, buildinfo.Info{Version: "test"})
	var document inspectJSONDocument
	if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", stdout.String(), err)
	}
	if strings.Count(strings.TrimSpace(stdout.String()), "\n") != 0 {
		t.Fatalf("JSON output is not one document line: %q", stdout.String())
	}
	return code, document, stdout.Bytes(), stderr.String()
}

type inspectJSONDocument struct {
	SchemaVersion string `json:"schema_version"`
	Result        struct {
		Outcome struct {
			Category     string `json:"category"`
			FailurePhase string `json:"failure_phase"`
			StatusCode   int    `json:"status_code"`
			Complete     bool   `json:"complete"`
		} `json:"outcome"`
		Body struct {
			TransferredBytes int64 `json:"transferred_bytes"`
			Complete         bool  `json:"complete"`
		} `json:"body"`
		Timings struct {
			Total phaseJSON `json:"total"`
			DNS   phaseJSON `json:"dns"`
		} `json:"timings"`
		Redirects struct {
			Hops []struct {
				Followed bool `json:"followed"`
			} `json:"hops"`
			FinalTarget *struct {
				Path string `json:"path"`
			} `json:"final_target"`
		} `json:"redirects"`
	} `json:"result"`
}

type phaseJSON struct {
	DurationMillis    *float64 `json:"duration_ms"`
	UnavailableReason string   `json:"unavailable_reason"`
}
