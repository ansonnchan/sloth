package httpmeasure

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http/httptrace"
	"sync"
	"testing"
	"time"
)

func TestTraceCollectorCapturesRepeatedAndOutOfOrderCallbacks(t *testing.T) {
	base := time.Unix(100, 0)
	next := sequenceClock(base)
	collector := newTraceCollector(next)
	trace := collector.ClientTrace()

	collector.MarkRequestStart()
	trace.DNSDone(httptrace.DNSDoneInfo{Coalesced: true, Err: &net.DNSError{Err: "failure"}})
	trace.DNSStart(httptrace.DNSStartInfo{Host: "example.com"})
	trace.ConnectStart("tcp6", "[::1]:443")
	trace.ConnectStart("tcp4", "127.0.0.1:443")
	trace.ConnectDone("tcp6", "[::1]:443", errors.New("refused"))
	trace.ConnectDone("tcp4", "127.0.0.1:443", nil)
	trace.TLSHandshakeStart()
	trace.TLSHandshakeDone(tls.ConnectionState{Version: tls.VersionTLS13, NegotiatedProtocol: "h2"}, nil)
	trace.GotConn(httptrace.GotConnInfo{Reused: true, WasIdle: true, IdleTime: time.Second})
	trace.WroteHeaders()
	trace.WroteRequest(httptrace.WroteRequestInfo{})
	trace.GotFirstResponseByte()
	collector.MarkResponseHeaders()
	collector.ObserveProtocol("HTTP/2.0")
	collector.MarkBodyDone()

	record := collector.Snapshot()
	if len(record.DNSStarts) != 1 || len(record.DNSDones) != 1 {
		t.Fatalf("DNS observations = (%d, %d), want (1, 1)", len(record.DNSStarts), len(record.DNSDones))
	}
	if len(record.ConnectStarts) != 2 || len(record.ConnectDones) != 2 {
		t.Fatalf("connect observations = (%d, %d), want (2, 2)", len(record.ConnectStarts), len(record.ConnectDones))
	}
	if record.ConnectDones[0].Error == nil || record.ConnectDones[1].Error != nil {
		t.Fatal("connect callback errors were not retained")
	}
	if len(record.TLSStarts) != 1 || len(record.TLSDones) != 1 || record.TLSDones[0].Negotiated != "h2" {
		t.Fatalf("TLS observations = %#v", record.TLSDones)
	}
	if len(record.Connections) != 1 || !record.Connections[0].Reused || !record.Connections[0].WasIdle {
		t.Fatalf("connection observation = %#v", record.Connections)
	}
	if len(record.WroteHeaders) != 1 || len(record.WroteRequests) != 1 || len(record.FirstResponseBytes) != 1 {
		t.Fatal("request lifecycle hooks were not retained")
	}
	if len(record.Protocols) != 1 || record.Protocols[0] != "HTTP/2.0" {
		t.Fatalf("protocols = %v", record.Protocols)
	}
	if record.RequestStart.IsZero() || record.ResponseHeaders.IsZero() || record.BodyDone.IsZero() {
		t.Fatal("request lifecycle markers were not retained")
	}
}

func TestTraceSnapshotIsDefensive(t *testing.T) {
	collector := newTraceCollector(sequenceClock(time.Unix(200, 0)))
	collector.ClientTrace().DNSStart(httptrace.DNSStartInfo{Host: "example.com"})
	collector.ObserveProtocol("HTTP/1.1")

	record := collector.Snapshot()
	record.DNSStarts[0].Host = "changed.example"
	record.Protocols[0] = "changed"

	second := collector.Snapshot()
	if second.DNSStarts[0].Host != "example.com" || second.Protocols[0] != "HTTP/1.1" {
		t.Fatalf("snapshot mutation changed collector: %#v", second)
	}
}

func TestTraceCollectorsDoNotShareState(t *testing.T) {
	first := NewTraceCollector()
	second := NewTraceCollector()
	const observations = 100

	var group sync.WaitGroup
	group.Add(2)
	go func() {
		defer group.Done()
		trace := first.ClientTrace()
		for range observations {
			trace.ConnectStart("tcp4", "127.0.0.1:80")
		}
	}()
	go func() {
		defer group.Done()
		trace := second.ClientTrace()
		for range observations {
			trace.DNSStart(httptrace.DNSStartInfo{Host: "example.com"})
		}
	}()
	group.Wait()

	firstRecord := first.Snapshot()
	secondRecord := second.Snapshot()
	if len(firstRecord.ConnectStarts) != observations || len(firstRecord.DNSStarts) != 0 {
		t.Fatalf("first collector mixed state: %#v", firstRecord)
	}
	if len(secondRecord.DNSStarts) != observations || len(secondRecord.ConnectStarts) != 0 {
		t.Fatalf("second collector mixed state: %#v", secondRecord)
	}
}

func TestTraceCallbackErrorsRemainTypedAndSafe(t *testing.T) {
	cause := &net.DNSError{Name: "example.com", Err: "no such host"}
	collector := NewTraceCollector()
	collector.ClientTrace().DNSDone(httptrace.DNSDoneInfo{Err: cause})

	observed := collector.Snapshot().DNSDones[0].Error
	if !errors.Is(observed, cause) {
		t.Fatal("trace error does not retain its cause")
	}
	if observed.Phase() != FailurePhaseDNS || observed.Error() != "DNS lookup failed" {
		t.Fatalf("trace error = (%q, %q)", observed.Phase(), observed.Error())
	}
}

func sequenceClock(start time.Time) func() time.Time {
	current := start
	return func() time.Time {
		result := current
		current = current.Add(time.Millisecond)
		return result
	}
}
