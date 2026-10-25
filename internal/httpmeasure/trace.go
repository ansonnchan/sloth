package httpmeasure

import (
	"crypto/tls"
	"net/http/httptrace"
	"sync"
	"time"
)

// DNSStartObservation records one DNS lookup start callback.
type DNSStartObservation struct {
	At   time.Time
	Host string
}

// DNSDoneObservation records one DNS lookup completion callback.
type DNSDoneObservation struct {
	At           time.Time
	AddressCount int
	Coalesced    bool
	Error        *RequestError
}

// ConnectStartObservation records one network connection attempt.
type ConnectStartObservation struct {
	At      time.Time
	Network string
}

// ConnectDoneObservation records one network connection completion callback.
type ConnectDoneObservation struct {
	At      time.Time
	Network string
	Error   *RequestError
}

// TLSStartObservation records one TLS handshake start callback.
type TLSStartObservation struct {
	At time.Time
}

// TLSDoneObservation records one TLS handshake completion callback.
type TLSDoneObservation struct {
	At         time.Time
	Version    uint16
	Negotiated string
	Error      *RequestError
}

// ConnectionObservation records the connection selected for the request.
type ConnectionObservation struct {
	At       time.Time
	Reused   bool
	WasIdle  bool
	IdleTime time.Duration
}

// WriteObservation records completion of writing one request.
type WriteObservation struct {
	At    time.Time
	Error *RequestError
}

// TraceRecord is an immutable snapshot of raw request-scoped callbacks. Event
// slices remain separate because hooks may repeat, overlap, or arrive without a
// matching counterpart.
type TraceRecord struct {
	RequestStart       time.Time
	ResponseHeaders    time.Time
	BodyDone           time.Time
	DNSStarts          []DNSStartObservation
	DNSDones           []DNSDoneObservation
	ConnectStarts      []ConnectStartObservation
	ConnectDones       []ConnectDoneObservation
	TLSStarts          []TLSStartObservation
	TLSDones           []TLSDoneObservation
	Connections        []ConnectionObservation
	WroteHeaders       []time.Time
	WroteRequests      []WriteObservation
	FirstResponseBytes []time.Time
	Protocols          []string
}

// TraceCollector owns callbacks for exactly one request attempt.
type TraceCollector struct {
	mu     sync.Mutex
	now    func() time.Time
	record TraceRecord
}

// NewTraceCollector creates a request-local collector using monotonic-capable
// time.Time values from time.Now.
func NewTraceCollector() *TraceCollector {
	return newTraceCollector(time.Now)
}

func newTraceCollector(now func() time.Time) *TraceCollector {
	return &TraceCollector{now: now}
}

// ClientTrace returns callbacks suitable for httptrace.WithClientTrace.
func (c *TraceCollector) ClientTrace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{
		DNSStart: func(info httptrace.DNSStartInfo) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.DNSStarts = append(c.record.DNSStarts, DNSStartObservation{At: c.now(), Host: info.Host})
		},
		DNSDone: func(info httptrace.DNSDoneInfo) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.DNSDones = append(c.record.DNSDones, DNSDoneObservation{
				At:           c.now(),
				AddressCount: len(info.Addrs),
				Coalesced:    info.Coalesced,
				Error:        observedError(info.Err),
			})
		},
		ConnectStart: func(network, _ string) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.ConnectStarts = append(c.record.ConnectStarts, ConnectStartObservation{At: c.now(), Network: network})
		},
		ConnectDone: func(network, _ string, err error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.ConnectDones = append(c.record.ConnectDones, ConnectDoneObservation{
				At:      c.now(),
				Network: network,
				Error:   observedError(err),
			})
		},
		TLSHandshakeStart: func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.TLSStarts = append(c.record.TLSStarts, TLSStartObservation{At: c.now()})
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.TLSDones = append(c.record.TLSDones, TLSDoneObservation{
				At:         c.now(),
				Version:    state.Version,
				Negotiated: state.NegotiatedProtocol,
				Error:      observedError(err),
			})
		},
		GotConn: func(info httptrace.GotConnInfo) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.Connections = append(c.record.Connections, ConnectionObservation{
				At:       c.now(),
				Reused:   info.Reused,
				WasIdle:  info.WasIdle,
				IdleTime: info.IdleTime,
			})
		},
		WroteHeaders: func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.WroteHeaders = append(c.record.WroteHeaders, c.now())
		},
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.WroteRequests = append(c.record.WroteRequests, WriteObservation{
				At:    c.now(),
				Error: observedError(info.Err),
			})
		},
		GotFirstResponseByte: func() {
			c.mu.Lock()
			defer c.mu.Unlock()
			c.record.FirstResponseBytes = append(c.record.FirstResponseBytes, c.now())
		},
	}
}

// MarkRequestStart records the instant immediately before RoundTrip begins.
func (c *TraceCollector) MarkRequestStart() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record.RequestStart = c.now()
}

// MarkResponseHeaders records when RoundTrip returns a response.
func (c *TraceCollector) MarkResponseHeaders() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record.ResponseHeaders = c.now()
}

// MarkBodyDone records body completion or a terminal body-read error.
func (c *TraceCollector) MarkBodyDone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record.BodyDone = c.now()
}

// ObserveProtocol retains the protocol reported by an HTTP response.
func (c *TraceCollector) ObserveProtocol(protocol string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.record.Protocols = append(c.record.Protocols, protocol)
}

// Snapshot returns a deep copy that cannot mutate the collector.
func (c *TraceCollector) Snapshot() TraceRecord {
	c.mu.Lock()
	defer c.mu.Unlock()

	record := c.record
	record.DNSStarts = append([]DNSStartObservation(nil), c.record.DNSStarts...)
	record.DNSDones = append([]DNSDoneObservation(nil), c.record.DNSDones...)
	record.ConnectStarts = append([]ConnectStartObservation(nil), c.record.ConnectStarts...)
	record.ConnectDones = append([]ConnectDoneObservation(nil), c.record.ConnectDones...)
	record.TLSStarts = append([]TLSStartObservation(nil), c.record.TLSStarts...)
	record.TLSDones = append([]TLSDoneObservation(nil), c.record.TLSDones...)
	record.Connections = append([]ConnectionObservation(nil), c.record.Connections...)
	record.WroteHeaders = append([]time.Time(nil), c.record.WroteHeaders...)
	record.WroteRequests = append([]WriteObservation(nil), c.record.WroteRequests...)
	record.FirstResponseBytes = append([]time.Time(nil), c.record.FirstResponseBytes...)
	record.Protocols = append([]string(nil), c.record.Protocols...)
	return record
}

func observedError(err error) *RequestError {
	if err == nil {
		return nil
	}
	return classifyRequestError(err)
}
