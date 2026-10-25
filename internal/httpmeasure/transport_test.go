package httpmeasure

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"testing"
	"time"
)

func TestTransportProfilesOwnSeparatePools(t *testing.T) {
	fresh, err := NewTransport(TransportFresh)
	if err != nil {
		t.Fatalf("NewTransport(fresh) error = %v", err)
	}
	defer fresh.Close()
	reuse, err := NewTransport(TransportReuse)
	if err != nil {
		t.Fatalf("NewTransport(reuse) error = %v", err)
	}
	defer reuse.Close()

	if fresh.transport == reuse.transport {
		t.Fatal("fresh and reuse profiles share a transport")
	}
	if !fresh.transport.DisableKeepAlives {
		t.Fatal("fresh profile allows keep-alive reuse")
	}
	if reuse.transport.DisableKeepAlives {
		t.Fatal("reuse profile disables keep-alives")
	}
	for name, transport := range map[string]*http.Transport{"fresh": fresh.transport, "reuse": reuse.transport} {
		if !transport.ForceAttemptHTTP2 || transport.Proxy == nil || transport.DialContext == nil {
			t.Fatalf("%s profile lacks explicit HTTP/2, proxy, or dial behavior", name)
		}
	}
}

func TestTransportProfilesControlObservedReuse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	defer server.Close()

	cases := []struct {
		name             string
		profile          TransportProfile
		wantSecondReused bool
	}{
		{name: "fresh", profile: TransportFresh, wantSecondReused: false},
		{name: "reuse", profile: TransportReuse, wantSecondReused: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			owner, err := NewTransport(test.profile)
			if err != nil {
				t.Fatalf("NewTransport() error = %v", err)
			}
			defer owner.Close()
			client := owner.Client(time.Second)

			_ = requestReuseState(t, client, server.URL)
			secondReused := requestReuseState(t, client, server.URL)
			if secondReused != test.wantSecondReused {
				t.Fatalf("second request reused = %v, want %v", secondReused, test.wantSecondReused)
			}
		})
	}
}

func TestTransportSupportsHTTPSAndHTTP2(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	base := server.Client().Transport.(*http.Transport)
	owner, err := newTransportOwner(TransportReuse, base)
	if err != nil {
		t.Fatalf("newTransportOwner() error = %v", err)
	}
	defer owner.Close()

	response, err := owner.Client(time.Second).Get(server.URL)
	if err != nil {
		t.Fatalf("GET HTTP/2 test server: %v", err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("finish response body: read %v, close %v", readErr, closeErr)
	}
	if response.ProtoMajor != 2 {
		t.Fatalf("protocol = %q, want HTTP/2", response.Proto)
	}
}

func TestTransportCloseClosesIdleConnections(t *testing.T) {
	closed := make(chan struct{}, 1)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			select {
			case closed <- struct{}{}:
			default:
			}
		}
	}
	server.Start()
	defer server.Close()

	owner, err := NewTransport(TransportReuse)
	if err != nil {
		t.Fatalf("NewTransport() error = %v", err)
	}
	client := owner.Client(time.Second)
	_ = requestReuseState(t, client, server.URL)
	owner.Close()
	owner.Close()

	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("owned idle connection was not closed")
	}
}

func TestTransportRejectsUnknownProfile(t *testing.T) {
	if _, err := NewTransport("unknown"); err == nil {
		t.Fatal("NewTransport(unknown) unexpectedly succeeded")
	}
}

func requestReuseState(t *testing.T, client *http.Client, target string) bool {
	t.Helper()
	var reused bool
	trace := &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) { reused = info.Reused }}
	request, err := http.NewRequestWithContext(httptrace.WithClientTrace(context.Background(), trace), http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("http.NewRequestWithContext() error = %v", err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("client.Do() error = %v", err)
	}
	_, readErr := io.Copy(io.Discard, response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("finish response body: read %v, close %v", readErr, closeErr)
	}
	return reused
}
