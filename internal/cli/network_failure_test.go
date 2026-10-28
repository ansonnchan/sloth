package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
)

func TestInspectCommandClassifiesLocalNetworkFailures(t *testing.T) {
	refused := unusedLoopbackAddress(t)
	tlsServer := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	}))
	tlsServer.Config.ErrorLog = log.New(io.Discard, "", 0)
	tlsServer.StartTLS()
	defer tlsServer.Close()
	timeoutServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		writer.WriteHeader(http.StatusOK)
	}))
	defer timeoutServer.Close()
	reset := oneShotTCPServer(t, "", true)
	partial := oneShotTCPServer(t, "HTTP/1.1 200 OK\r\nContent-Length: 100\r\nConnection: close\r\n\r\nshort", false)
	redirectServer := httptest.NewServer(nil)
	redirectServer.Config.Handler = http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, redirectServer.URL+"/loop?token=redirect-secret", http.StatusFound)
	})
	defer redirectServer.Close()

	tests := []struct {
		name       string
		args       []string
		category   httpmeasure.OutcomeCategory
		phase      httpmeasure.FailurePhase
		wantStatus int
	}{
		{"refused connection", []string{"--timeout", "300ms", "http://" + refused + "/?token=refused-secret"}, httpmeasure.OutcomeTransportError, httpmeasure.FailurePhaseConnect, 0},
		{"TLS verification", []string{"--timeout", "300ms", tlsServer.URL + "/?token=tls-secret"}, httpmeasure.OutcomeTransportError, httpmeasure.FailurePhaseTLS, 0},
		{"timeout", []string{"--timeout", "20ms", timeoutServer.URL + "/?token=timeout-secret"}, httpmeasure.OutcomeTimeout, httpmeasure.FailurePhaseUnknown, 0},
		{"connection reset", []string{"--timeout", "300ms", "http://" + reset + "/?token=reset-secret"}, httpmeasure.OutcomeTransportError, httpmeasure.FailurePhaseUnknown, 0},
		{"partial body", []string{"--timeout", "300ms", "http://" + partial + "/?token=partial-secret"}, httpmeasure.OutcomeTransportError, httpmeasure.FailurePhaseBody, http.StatusOK},
		{"redirect loop", []string{"--timeout", "300ms", "--follow-redirects", "--redirect-limit", "2", redirectServer.URL + "/loop?token=source-secret"}, httpmeasure.OutcomeTransportError, httpmeasure.FailurePhaseUnknown, -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string{"inspect", "--json"}, test.args...)
			var stdout, stderr bytes.Buffer
			code := Run(args, &stdout, &stderr, buildinfo.Info{})
			if code != int(ExitRuntime) {
				t.Fatalf("Run() = %d, streams = (%q, %q)", code, stdout.String(), stderr.String())
			}
			var document inspect.Document
			if err := json.Unmarshal(stdout.Bytes(), &document); err != nil {
				t.Fatalf("partial JSON = %q: %v", stdout.String(), err)
			}
			if document.Result.Outcome.Category != test.category || (test.wantStatus >= 0 && document.Result.Outcome.StatusCode != test.wantStatus) {
				t.Fatalf("outcome = %+v", document.Result.Outcome)
			}
			if test.phase != httpmeasure.FailurePhaseUnknown && document.Result.Outcome.FailurePhase != test.phase {
				t.Fatalf("failure phase = %q, want %q", document.Result.Outcome.FailurePhase, test.phase)
			}
			for _, secret := range []string{"refused-secret", "tls-secret", "timeout-secret", "reset-secret", "partial-secret", "source-secret", "redirect-secret"} {
				if strings.Contains(stdout.String()+stderr.String(), secret) {
					t.Fatalf("secret %q escaped: %s %s", secret, stdout.String(), stderr.String())
				}
			}
		})
	}
}

func TestInspectCancellationReturnsWithinBound(t *testing.T) {
	started := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, request *http.Request) {
		close(started)
		<-request.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var stdout, stderr bytes.Buffer
	done := make(chan int, 1)
	go func() {
		done <- RunContext(ctx, []string{"inspect", "--json", "--timeout", "5s", server.URL}, &stdout, &stderr, buildinfo.Info{})
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("request did not start")
	}
	cancelledAt := time.Now()
	cancel()
	select {
	case code := <-done:
		if code != int(ExitInterrupted) {
			t.Fatalf("RunContext() = %d, stderr = %q", code, stderr.String())
		}
		if elapsed := time.Since(cancelledAt); elapsed > time.Second {
			t.Fatalf("cancellation latency = %s, want <= 1s", elapsed)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation exceeded 1s bound")
	}
}

func unusedLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return address
}

func oneShotTCPServer(t *testing.T, response string, reset bool) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		_ = connection.SetDeadline(time.Now().Add(time.Second))
		if reset {
			if tcp, ok := connection.(*net.TCPConn); ok {
				_ = tcp.SetLinger(0)
			}
			return
		}
		reader := bufio.NewReader(connection)
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil || line == "\r\n" {
				break
			}
		}
		_, _ = io.WriteString(connection, response)
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	return listener.Addr().String()
}
