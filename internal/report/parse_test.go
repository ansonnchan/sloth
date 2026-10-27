package report

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAcceptsUnknownCompatibleFieldsAndPreservesNull(t *testing.T) {
	data := reportJSON(t)
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	object["schema_version"] = "sloth.report/v1.1"
	object["future_field"] = map[string]any{"value": 1}
	data, _ = json.Marshal(object)
	parsed, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.NewerMinor || parsed.Report.SchemaVersion != "sloth.report/v1.1" || parsed.UnknownFields["future_field"] == nil {
		t.Fatalf("parsed = %+v", parsed)
	}
	latency := parsed.Report.Experiments[0].Aggregate.SchedulingDelay
	if latency.MinMillis != nil || latency.Count != 0 {
		t.Fatalf("unavailable latency lost null semantics: %+v", latency)
	}
}

func TestReadErrorKinds(t *testing.T) {
	directory := t.TempDir()
	valid := reportJSON(t)
	tests := []struct {
		name string
		data []byte
		kind ReadErrorKind
	}{
		{name: "malformed", data: []byte(`{"schema_version": nope}`), kind: ReadMalformed},
		{name: "truncated", data: []byte(`{"schema_version":"sloth.report/v1.0"`), kind: ReadTruncated},
		{name: "trailing", data: append(append([]byte(nil), valid...), []byte(` {}`)...), kind: ReadMalformed},
		{name: "unsupported", data: replaceVersion(t, valid, "sloth.report/v2.0"), kind: ReadUnsupportedMajor},
		{name: "invalid", data: replaceVersion(t, valid, "invalid"), kind: ReadValidation},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(directory, test.name+".json")
			if err := os.WriteFile(path, test.data, 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Read(path)
			assertReadKind(t, err, test.kind)
		})
	}
	_, err := Read(filepath.Join(directory, "missing.json"))
	assertReadKind(t, err, ReadMissing)
}

func TestParseRejectsOversizedInput(t *testing.T) {
	_, err := Parse(make([]byte, MaxReportBytes+1))
	assertReadKind(t, err, ReadOversized)
}

func TestReadRejectsOversizedFileBeforeDecode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(MaxReportBytes + 1); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	_, err = Read(path)
	assertReadKind(t, err, ReadOversized)
}

func FuzzParseReport(f *testing.F) {
	f.Add(reportJSON(f))
	f.Add([]byte(`{"schema_version":"sloth.report/v1.0"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if int64(len(data)) > MaxReportBytes+1 {
			t.Skip()
		}
		_, _ = Parse(data)
	})
}

type testingFatal interface {
	Helper()
	Fatal(args ...any)
}

func reportJSON(t testingFatal) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "docs", "examples", "report-v1.aggregate.json"))
	if err != nil {
		t.Fatal(err)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		t.Fatal(err)
	}
	// Add one aggregate so null preservation is exercised.
	object["experiments"] = []any{map[string]any{
		"id": "baseline", "kind": "baseline", "requested_samples": 1.0, "warmup_requests": 0.0,
		"workers": 1.0, "rate_per_second": 1.0, "warmup_outcomes": map[string]any{},
		"execution": map[string]any{
			"started_at": "2026-10-27T12:00:00Z", "finished_at": "2026-10-27T12:00:01Z",
			"offered_starts": 1.0, "achieved_starts": 1.0, "missed_arrivals": 0.0, "interrupted": false,
		},
		"aggregate": map[string]any{
			"outcomes":       map[string]any{"attempted": 1.0, "completed": 1.0, "successful": 1.0, "http_error": 0.0, "transport_error": 0.0, "timeout": 0.0, "cancelled": 0.0, "body_limit": 0.0},
			"status_classes": map[string]any{"2xx": 1.0}, "response_bytes": 0.0,
			"latency": statisticsJSON(1), "scheduling_delay": statisticsJSON(0), "phases": map[string]any{},
			"rates":       map[string]any{"duration_ms": 1000.0, "offered_starts": 1.0, "achieved_starts": 1.0, "offered_per_second": 1.0, "achieved_per_second": 1.0, "completed_per_second": 1.0},
			"concurrency": map[string]any{"peak": 1.0, "time_weighted_in_flight": 1.0},
			"protocols":   map[string]any{"HTTP/1.1": 1.0}, "connections": map[string]any{"observed": 1.0, "new": 1.0, "reused": 0.0}, "interrupted": false,
		},
		"limitations": []any{},
	}}
	encoded, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func statisticsJSON(count float64) map[string]any {
	value := any(nil)
	if count > 0 {
		value = 1.0
	}
	return map[string]any{
		"count": count, "min_ms": value, "max_ms": value, "mean_ms": value, "p50_ms": value,
		"p90_ms": value, "p95_ms": value, "p99_ms": value, "population_stddev_ms": value,
	}
}

func replaceVersion(t *testing.T, data []byte, version string) []byte {
	t.Helper()
	return []byte(strings.Replace(string(data), "sloth.report/v1.0", version, 1))
}

func assertReadKind(t *testing.T, err error, want ReadErrorKind) {
	t.Helper()
	var readErr *ReadError
	if !errors.As(err, &readErr) || readErr.Kind != want {
		t.Fatalf("error = %v, want kind %s", err, want)
	}
}
