package httpmeasure

import (
	"encoding/json"
	"sort"
	"time"
)

// UnavailableReason explains why a timing phase has no defensible value.
type UnavailableReason string

const (
	UnavailableMissingStart    UnavailableReason = "missing_start"
	UnavailableMissingEnd      UnavailableReason = "missing_end"
	UnavailableIncomplete      UnavailableReason = "incomplete_observations"
	UnavailableOutOfOrder      UnavailableReason = "out_of_order_observations"
	UnavailableConnectionReuse UnavailableReason = "connection_reused"
	UnavailableCleartext       UnavailableReason = "cleartext_request"
)

// PhaseDuration distinguishes an observed zero duration from an unavailable
// measurement and carries a stable reason for unavailable values.
type PhaseDuration struct {
	value  time.Duration
	reason UnavailableReason
}

// TimingPhases contains independently derived client-observable intervals.
// The phases are not required to be disjoint or add up to Total.
type TimingPhases struct {
	Total         PhaseDuration `json:"total"`
	DNS           PhaseDuration `json:"dns"`
	Connect       PhaseDuration `json:"connect"`
	TLS           PhaseDuration `json:"tls"`
	RequestWrite  PhaseDuration `json:"request_write"`
	FirstByteWait PhaseDuration `json:"first_byte_wait"`
	BodyTransfer  PhaseDuration `json:"body_transfer"`
}

// Available reports whether the duration was observed.
func (p PhaseDuration) Available() bool { return p.reason == "" }

// Value returns the duration and whether it is available.
func (p PhaseDuration) Value() (time.Duration, bool) { return p.value, p.Available() }

// Reason returns why an unavailable phase could not be measured.
func (p PhaseDuration) Reason() UnavailableReason { return p.reason }

// MarshalJSON preserves the difference between an observed zero and an
// unavailable phase by encoding duration_ms as a number or null.
func (p PhaseDuration) MarshalJSON() ([]byte, error) {
	type representation struct {
		DurationMillis    *float64          `json:"duration_ms"`
		UnavailableReason UnavailableReason `json:"unavailable_reason,omitempty"`
	}
	if !p.Available() {
		return json.Marshal(representation{UnavailableReason: p.reason})
	}
	milliseconds := float64(p.value) / float64(time.Millisecond)
	return json.Marshal(representation{DurationMillis: &milliseconds})
}

// DeriveTimingPhases converts raw callbacks into conservative timing values.
// secure must reflect whether the request used TLS.
func DeriveTimingPhases(record TraceRecord, secure bool) TimingPhases {
	reused := len(record.Connections) > 0 && record.Connections[0].Reused

	phases := TimingPhases{
		Total:         durationBetween(record.RequestStart, record.BodyDone),
		RequestWrite:  intervalDuration(writeHeaderTimes(record), writeDoneTimes(record)),
		FirstByteWait: intervalDuration(writeDoneTimes(record), record.FirstResponseBytes),
		BodyTransfer:  durationBetween(lastTime(record.FirstResponseBytes), record.BodyDone),
	}

	if reused {
		phases.DNS = unavailable(UnavailableConnectionReuse)
		phases.Connect = unavailable(UnavailableConnectionReuse)
		phases.TLS = unavailable(UnavailableConnectionReuse)
		return phases
	}

	phases.DNS = intervalDuration(dnsStartTimes(record), dnsDoneTimes(record))
	phases.Connect = intervalDuration(connectStartTimes(record), connectDoneTimes(record))
	if secure {
		phases.TLS = intervalDuration(tlsStartTimes(record), tlsDoneTimes(record))
	} else {
		phases.TLS = unavailable(UnavailableCleartext)
	}
	return phases
}

func durationBetween(start, end time.Time) PhaseDuration {
	if start.IsZero() {
		return unavailable(UnavailableMissingStart)
	}
	if end.IsZero() {
		return unavailable(UnavailableMissingEnd)
	}
	if end.Before(start) {
		return unavailable(UnavailableOutOfOrder)
	}
	return measured(end.Sub(start))
}

func intervalDuration(starts, ends []time.Time) PhaseDuration {
	if len(starts) == 0 {
		return unavailable(UnavailableMissingStart)
	}
	if len(ends) == 0 {
		return unavailable(UnavailableMissingEnd)
	}
	if len(starts) != len(ends) {
		return unavailable(UnavailableIncomplete)
	}

	starts = append([]time.Time(nil), starts...)
	ends = append([]time.Time(nil), ends...)
	sort.Slice(starts, func(i, j int) bool { return starts[i].Before(starts[j]) })
	sort.Slice(ends, func(i, j int) bool { return ends[i].Before(ends[j]) })

	type interval struct{ start, end time.Time }
	intervals := make([]interval, len(starts))
	for index := range starts {
		if ends[index].Before(starts[index]) {
			return unavailable(UnavailableOutOfOrder)
		}
		intervals[index] = interval{start: starts[index], end: ends[index]}
	}

	current := intervals[0]
	var total time.Duration
	for _, candidate := range intervals[1:] {
		if !candidate.start.After(current.end) {
			if candidate.end.After(current.end) {
				current.end = candidate.end
			}
			continue
		}
		total += current.end.Sub(current.start)
		current = candidate
	}
	total += current.end.Sub(current.start)
	return measured(total)
}

func measured(value time.Duration) PhaseDuration { return PhaseDuration{value: value} }

func unavailable(reason UnavailableReason) PhaseDuration { return PhaseDuration{reason: reason} }

func lastTime(values []time.Time) time.Time {
	if len(values) == 0 {
		return time.Time{}
	}
	latest := values[0]
	for _, value := range values[1:] {
		if value.After(latest) {
			latest = value
		}
	}
	return latest
}

func dnsStartTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.DNSStarts))
	for index, observation := range record.DNSStarts {
		result[index] = observation.At
	}
	return result
}

func dnsDoneTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.DNSDones))
	for index, observation := range record.DNSDones {
		result[index] = observation.At
	}
	return result
}

func connectStartTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.ConnectStarts))
	for index, observation := range record.ConnectStarts {
		result[index] = observation.At
	}
	return result
}

func connectDoneTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.ConnectDones))
	for index, observation := range record.ConnectDones {
		result[index] = observation.At
	}
	return result
}

func tlsStartTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.TLSStarts))
	for index, observation := range record.TLSStarts {
		result[index] = observation.At
	}
	return result
}

func tlsDoneTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.TLSDones))
	for index, observation := range record.TLSDones {
		result[index] = observation.At
	}
	return result
}

func writeHeaderTimes(record TraceRecord) []time.Time {
	return append([]time.Time(nil), record.WroteHeaders...)
}

func writeDoneTimes(record TraceRecord) []time.Time {
	result := make([]time.Time, len(record.WroteRequests))
	for index, observation := range record.WroteRequests {
		result[index] = observation.At
	}
	return result
}
