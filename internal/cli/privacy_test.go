package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestInspectPrivacyCanariesReachServerButNeverEscapeOutput(t *testing.T) {
	const authorization = "sloth-auth-canary-7f311"
	const cookie = "sloth-cookie-canary-8e422"
	const query = "sloth-query-canary-9d533"
	const body = "sloth-body-canary-af644"
	received := &privacyRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		contents, _ := io.ReadAll(request.Body)
		received.record(request.Header.Get("Authorization"), request.Header.Get("Cookie"), request.URL.Query().Get("token"), string(contents))
		writer.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	bodyPath := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(bodyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"inspect", "--json", "--method", "POST", "--allow-unsafe-methods",
		"--header", "Authorization: " + authorization, "--header", "Cookie: session=" + cookie,
		"--body-file", bodyPath, server.URL + "/inspect?token=" + query,
	}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	gotAuthorization, gotCookie, gotQuery, gotBody := received.snapshot()
	if gotAuthorization != authorization || gotCookie != "session="+cookie || gotQuery != query || gotBody != body {
		t.Fatalf("server did not receive seeded canaries: %q %q %q %q", gotAuthorization, gotCookie, gotQuery, gotBody)
	}
	assertCanariesAbsent(t, stdout.String()+stderr.String(), authorization, cookie, query, body)
	if !strings.Contains(stdout.String(), "REDACTED") {
		t.Fatalf("inspect JSON does not mark redaction: %s", stdout.String())
	}
}

func TestExportReportAndSamplesExcludeRuntimeSecrets(t *testing.T) {
	const authorization = "sloth-export-auth-canary-b1755"
	const cookie = "sloth-export-cookie-canary-c2866"
	const query = "sloth-export-query-canary-d3977"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	path := filepath.Join(t.TempDir(), "report.json")
	var stdout, stderr bytes.Buffer
	code := Run([]string{
		"export", "--quiet", "--output", path, "--requests", "10", "--warmups", "0",
		"--rate", "1000", "--concurrency", "1,2", "--allow-high-load", "--include-samples", "--sample-limit", "10",
		"--header", "Authorization: " + authorization, "--header", "Cookie: session=" + cookie,
		server.URL + "/export?token=" + query,
	}, &stdout, &stderr, buildinfo.Info{Version: "privacy-test"})
	if code != int(ExitSuccess) {
		t.Fatalf("Run() = %d, stderr = %q", code, stderr.String())
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertCanariesAbsent(t, string(data)+stdout.String()+stderr.String(), authorization, cookie, query)
	if !bytes.Contains(data, []byte("\"samples\"")) || !bytes.Contains(data, []byte("%5BREDACTED%5D")) {
		t.Fatalf("report lacks bounded samples or redacted identity: %s", data)
	}
}

func TestSensitiveTargetErrorsAndDebugFormattingAreRedacted(t *testing.T) {
	const credential = "sloth-url-credential-canary-e4088"
	var stdout, stderr bytes.Buffer
	code := Run([]string{"inspect", "https://user:" + credential + "@example.com/private"}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitUsage) {
		t.Fatalf("credential Run() = %d, stderr = %q", code, stderr.String())
	}
	assertCanariesAbsent(t, stdout.String()+stderr.String(), credential)

	const query = "sloth-transport-query-canary-f5199"
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"inspect", "--timeout", "100ms", "http://" + address + "/private?token=" + query}, &stdout, &stderr, buildinfo.Info{})
	if code != int(ExitRuntime) {
		t.Fatalf("transport Run() = %d, stderr = %q", code, stderr.String())
	}
	assertCanariesAbsent(t, stdout.String()+stderr.String(), query)

	const customHeader = "sloth-custom-header-canary-061aa"
	spec, err := httpmeasure.NewRequestSpec(httpmeasure.RequestSpecInput{
		Target: "https://example.com/path?token=" + query, Method: http.MethodGet,
		Headers: http.Header{"X-Api-Key": []string{customHeader}}, SecretHeaderNames: []string{"X-Api-Key"},
		Options: httpmeasure.RequestOptions{Timeout: time.Second, RedirectLimit: 5, BodyLimit: 1024},
	})
	if err != nil {
		t.Fatal(err)
	}
	metadata, _ := json.Marshal(spec.Metadata())
	formatted := fmt.Sprintf("%s %#v %+v", spec, spec, spec)
	assertCanariesAbsent(t, string(metadata)+formatted, query, customHeader)
	if !strings.Contains(string(metadata)+formatted, "REDACTED") {
		t.Fatalf("safe formatting does not show redaction: %s %s", metadata, formatted)
	}
}

type privacyRequest struct {
	mu                                 sync.Mutex
	authorization, cookie, query, body string
}

func (request *privacyRequest) record(authorization, cookie, query, body string) {
	request.mu.Lock()
	defer request.mu.Unlock()
	request.authorization, request.cookie, request.query, request.body = authorization, cookie, query, body
}

func (request *privacyRequest) snapshot() (string, string, string, string) {
	request.mu.Lock()
	defer request.mu.Unlock()
	return request.authorization, request.cookie, request.query, request.body
}

func assertCanariesAbsent(t *testing.T, output string, canaries ...string) {
	t.Helper()
	for _, canary := range canaries {
		if strings.Contains(output, canary) {
			t.Fatalf("privacy canary %q escaped into output: %s", canary, output)
		}
	}
}
