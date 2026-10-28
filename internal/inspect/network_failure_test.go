package inspect

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ansonnchan/sloth/internal/httpmeasure"
)

func TestServiceClassifiesInjectedDNSFailure(t *testing.T) {
	transport := &http.Transport{DialContext: func(context.Context, string, string) (net.Conn, error) {
		return nil, &net.DNSError{Err: "synthetic lookup failure", Name: "private.invalid", IsNotFound: true}
	}}
	factory := func(settings ClientSettings) (ClientSession, error) {
		return ClientSession{Client: &http.Client{Transport: transport, Timeout: settings.Timeout}, Close: transport.CloseIdleConnections}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{Target: "http://private.invalid/?token=dns-secret", Method: http.MethodGet, Options: defaultRequestOptions()})
	result, err := NewService(factory).Inspect(context.Background(), spec)
	if err == nil || result.Outcome.Category != httpmeasure.OutcomeTransportError || result.Outcome.FailurePhase != httpmeasure.FailurePhaseDNS {
		t.Fatalf("Inspect() = (%+v, %v)", result.Outcome, err)
	}
	if strings.Contains(err.Error(), "dns-secret") || strings.Contains(err.Error(), "private.invalid") {
		t.Fatalf("DNS error exposed raw target: %v", err)
	}
}

func TestServiceUsesConfiguredLocalProxy(t *testing.T) {
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		if !request.URL.IsAbs() || request.URL.Host != "backend.invalid" {
			t.Errorf("proxy request URL = %s", request.URL)
		}
		writer.WriteHeader(http.StatusNoContent)
	}))
	defer proxy.Close()
	proxyURL, err := url.Parse(proxy.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	factory := func(settings ClientSettings) (ClientSession, error) {
		client := &http.Client{Transport: transport, Timeout: settings.Timeout, CheckRedirect: settings.CheckRedirect}
		return ClientSession{Client: client, Close: transport.CloseIdleConnections}, nil
	}
	spec := mustSpec(t, httpmeasure.RequestSpecInput{
		Target: "http://backend.invalid/path?token=proxy-secret", Method: http.MethodGet,
		Options: httpmeasure.RequestOptions{Timeout: time.Second, RedirectLimit: 5, BodyLimit: 1024},
	})
	result, err := NewService(factory).Inspect(context.Background(), spec)
	if err != nil || result.Outcome.Category != httpmeasure.OutcomeSuccess || requests.Load() != 1 {
		t.Fatalf("Inspect() = (%+v, %v), proxy requests = %d", result.Outcome, err, requests.Load())
	}
	if strings.Contains(result.Request.Target.RedactedURL, "proxy-secret") {
		t.Fatalf("proxy result contains secret: %+v", result.Request.Target)
	}
}
