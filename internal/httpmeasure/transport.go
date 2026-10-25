package httpmeasure

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// TransportProfile defines connection-pool behavior for a measurement arm.
type TransportProfile string

const (
	TransportDefault TransportProfile = "default"
	TransportFresh   TransportProfile = "fresh"
	TransportReuse   TransportProfile = "reuse"
)

// TransportOwner gives one experiment arm exclusive ownership of a transport
// and its connection pool.
type TransportOwner struct {
	transport *http.Transport
	closeOnce sync.Once
}

// NewTransport creates an isolated transport with explicit standard-library
// behavior for proxies, dual-stack dialing, TLS, HTTP/2, and idle pooling.
func NewTransport(profile TransportProfile) (*TransportOwner, error) {
	return newTransportOwner(profile, standardTransport())
}

func newTransportOwner(profile TransportProfile, base *http.Transport) (*TransportOwner, error) {
	if base == nil {
		return nil, errors.New("create HTTP transport: base transport is required")
	}
	if profile != TransportDefault && profile != TransportFresh && profile != TransportReuse {
		return nil, errors.New("create HTTP transport: unknown profile")
	}

	transport := base.Clone()
	transport.ForceAttemptHTTP2 = true
	if profile == TransportFresh {
		transport.DisableKeepAlives = true
		transport.MaxIdleConns = 0
		transport.MaxIdleConnsPerHost = -1
	} else {
		transport.DisableKeepAlives = false
		if transport.MaxIdleConns <= 0 {
			transport.MaxIdleConns = 100
		}
		if transport.MaxIdleConnsPerHost <= 0 {
			transport.MaxIdleConnsPerHost = 10
		}
	}
	return &TransportOwner{transport: transport}, nil
}

// RoundTripper returns the owned transport for an HTTP client.
func (o *TransportOwner) RoundTripper() http.RoundTripper { return o.transport }

// Client creates an HTTP client that uses the owned pool and timeout.
func (o *TransportOwner) Client(timeout time.Duration) *http.Client {
	return &http.Client{Transport: o.transport, Timeout: timeout}
}

// Close closes every idle connection owned by this transport. It is safe to
// call more than once.
func (o *TransportOwner) Close() {
	o.closeOnce.Do(o.transport.CloseIdleConnections)
}

func standardTransport() *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
}
