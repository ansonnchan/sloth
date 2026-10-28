package httpmeasure

import (
	"crypto/tls"
	"net"
	"net/http/httptrace"
	"testing"
	"time"
)

// BenchmarkTraceCollector measures only Sloth's request-local tracing and
// phase derivation. It deliberately performs no network I/O.
func BenchmarkTraceCollector(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		collector := NewTraceCollector()
		trace := collector.ClientTrace()
		collector.MarkRequestStart()
		trace.DNSStart(httptrace.DNSStartInfo{Host: "example.test"})
		trace.DNSDone(httptrace.DNSDoneInfo{Addrs: []net.IPAddr{{IP: net.IPv4(127, 0, 0, 1)}}})
		trace.ConnectStart("tcp", "127.0.0.1:443")
		trace.ConnectDone("tcp", "127.0.0.1:443", nil)
		trace.TLSHandshakeStart()
		trace.TLSHandshakeDone(tls.ConnectionState{Version: tls.VersionTLS13, NegotiatedProtocol: "h2"}, nil)
		trace.WroteHeaders()
		trace.WroteRequest(httptrace.WroteRequestInfo{})
		trace.GotFirstResponseByte()
		collector.MarkResponseHeaders()
		collector.MarkBodyDone()
		collector.ObserveProtocol("HTTP/2.0")
		record := collector.Snapshot()
		_ = DeriveTimingPhases(record, true)
	}
}

func BenchmarkDeriveTimingPhases(b *testing.B) {
	start := time.Unix(100, 0)
	record := TraceRecord{
		RequestStart:       start,
		BodyDone:           start.Add(12 * time.Millisecond),
		DNSStarts:          []DNSStartObservation{{At: start}},
		DNSDones:           []DNSDoneObservation{{At: start.Add(time.Millisecond)}},
		ConnectStarts:      []ConnectStartObservation{{At: start.Add(time.Millisecond)}},
		ConnectDones:       []ConnectDoneObservation{{At: start.Add(3 * time.Millisecond)}},
		TLSStarts:          []TLSStartObservation{{At: start.Add(3 * time.Millisecond)}},
		TLSDones:           []TLSDoneObservation{{At: start.Add(5 * time.Millisecond)}},
		WroteHeaders:       []time.Time{start.Add(5 * time.Millisecond)},
		WroteRequests:      []WriteObservation{{At: start.Add(6 * time.Millisecond)}},
		FirstResponseBytes: []time.Time{start.Add(10 * time.Millisecond)},
	}
	b.ReportAllocs()
	for b.Loop() {
		_ = DeriveTimingPhases(record, true)
	}
}
