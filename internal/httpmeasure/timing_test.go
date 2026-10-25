package httpmeasure

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDeriveTimingPhases(t *testing.T) {
	base := time.Unix(500, 0)
	record := TraceRecord{
		RequestStart: base,
		BodyDone:     base.Add(100 * time.Millisecond),
		DNSStarts: []DNSStartObservation{
			{At: base.Add(5 * time.Millisecond)},
			{At: base.Add(10 * time.Millisecond)},
		},
		DNSDones: []DNSDoneObservation{
			{At: base.Add(20 * time.Millisecond)},
			{At: base.Add(30 * time.Millisecond)},
		},
		ConnectStarts:      []ConnectStartObservation{{At: base.Add(30 * time.Millisecond)}},
		ConnectDones:       []ConnectDoneObservation{{At: base.Add(50 * time.Millisecond)}},
		TLSStarts:          []TLSStartObservation{{At: base.Add(50 * time.Millisecond)}},
		TLSDones:           []TLSDoneObservation{{At: base.Add(70 * time.Millisecond)}},
		Connections:        []ConnectionObservation{{At: base.Add(70 * time.Millisecond)}},
		WroteHeaders:       []time.Time{base.Add(72 * time.Millisecond)},
		WroteRequests:      []WriteObservation{{At: base.Add(75 * time.Millisecond)}},
		FirstResponseBytes: []time.Time{base.Add(90 * time.Millisecond)},
	}

	phases := DeriveTimingPhases(record, true)
	assertDuration(t, phases.Total, 100*time.Millisecond)
	assertDuration(t, phases.DNS, 25*time.Millisecond)
	assertDuration(t, phases.Connect, 20*time.Millisecond)
	assertDuration(t, phases.TLS, 20*time.Millisecond)
	assertDuration(t, phases.RequestWrite, 3*time.Millisecond)
	assertDuration(t, phases.FirstByteWait, 15*time.Millisecond)
	assertDuration(t, phases.BodyTransfer, 10*time.Millisecond)
}

func TestDeriveTimingPhasesHandlesSequentialRepeatedIntervals(t *testing.T) {
	base := time.Unix(600, 0)
	phase := intervalDuration(
		[]time.Time{base, base.Add(20 * time.Millisecond)},
		[]time.Time{base.Add(10 * time.Millisecond), base.Add(35 * time.Millisecond)},
	)
	assertDuration(t, phase, 25*time.Millisecond)
}

func TestReusedConnectionMakesSetupPhasesInapplicable(t *testing.T) {
	base := time.Unix(700, 0)
	phases := DeriveTimingPhases(TraceRecord{
		RequestStart: base,
		BodyDone:     base.Add(time.Millisecond),
		Connections:  []ConnectionObservation{{At: base, Reused: true}},
	}, true)

	for name, phase := range map[string]PhaseDuration{
		"DNS": phases.DNS, "connect": phases.Connect, "TLS": phases.TLS,
	} {
		assertUnavailable(t, name, phase, UnavailableConnectionReuse)
	}
}

func TestCleartextMakesTLSInapplicable(t *testing.T) {
	phases := DeriveTimingPhases(TraceRecord{}, false)
	assertUnavailable(t, "TLS", phases.TLS, UnavailableCleartext)
}

func TestIncompleteAndOutOfOrderEventsAreUnavailable(t *testing.T) {
	base := time.Unix(800, 0)
	cases := []struct {
		name   string
		starts []time.Time
		ends   []time.Time
		reason UnavailableReason
	}{
		{name: "missing start", ends: []time.Time{base}, reason: UnavailableMissingStart},
		{name: "missing end", starts: []time.Time{base}, reason: UnavailableMissingEnd},
		{name: "unequal events", starts: []time.Time{base, base.Add(time.Millisecond)}, ends: []time.Time{base.Add(2 * time.Millisecond)}, reason: UnavailableIncomplete},
		{name: "negative", starts: []time.Time{base.Add(time.Millisecond)}, ends: []time.Time{base}, reason: UnavailableOutOfOrder},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertUnavailable(t, test.name, intervalDuration(test.starts, test.ends), test.reason)
		})
	}
}

func TestPhaseDurationJSONDistinguishesZeroAndUnavailable(t *testing.T) {
	encodedZero, err := json.Marshal(measured(0))
	if err != nil {
		t.Fatalf("json.Marshal(zero) error = %v", err)
	}
	encodedUnavailable, err := json.Marshal(unavailable(UnavailableMissingStart))
	if err != nil {
		t.Fatalf("json.Marshal(unavailable) error = %v", err)
	}

	if !strings.Contains(string(encodedZero), `"duration_ms":0`) {
		t.Fatalf("zero JSON = %s", encodedZero)
	}
	if !strings.Contains(string(encodedUnavailable), `"duration_ms":null`) || !strings.Contains(string(encodedUnavailable), string(UnavailableMissingStart)) {
		t.Fatalf("unavailable JSON = %s", encodedUnavailable)
	}
}

func assertDuration(t *testing.T, phase PhaseDuration, want time.Duration) {
	t.Helper()
	got, ok := phase.Value()
	if !ok || got != want {
		t.Fatalf("duration = (%s, %v), want (%s, true), reason %q", got, ok, want, phase.Reason())
	}
}

func assertUnavailable(t *testing.T, name string, phase PhaseDuration, want UnavailableReason) {
	t.Helper()
	if _, ok := phase.Value(); ok || phase.Reason() != want {
		t.Fatalf("%s = (available %v, reason %q), want unavailable reason %q", name, ok, phase.Reason(), want)
	}
}
