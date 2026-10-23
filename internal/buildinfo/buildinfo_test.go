package buildinfo

import (
	"encoding/json"
	"testing"
)

func TestResolveUsesDevelopmentFallbacks(t *testing.T) {
	got := resolve("", " ", "", "not-a-bool")
	want := Info{Version: "dev", Commit: "unknown", BuildTime: "unknown", Dirty: false}
	if got != want {
		t.Fatalf("resolve() = %+v, want %+v", got, want)
	}
}

func TestResolveParsesDirtyValues(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "dirty"} {
		if got := resolve("v1.0.0", "abc123", "2026-10-23T20:00:00Z", value); !got.Dirty {
			t.Fatalf("resolve dirty %q = false", value)
		}
	}
}

func TestDisplay(t *testing.T) {
	tests := []struct {
		name string
		info Info
		want string
	}{
		{"development", Info{}, "dev"},
		{"release", Info{Version: "v1.2.3", Commit: "abc123", BuildTime: "2026-10-23T20:00:00Z"}, "v1.2.3 (commit abc123, built 2026-10-23T20:00:00Z)"},
		{"dirty", Info{Version: "dev", Commit: "abc123", Dirty: true}, "dev (commit abc123, dirty)"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.info.Display(); got != test.want {
				t.Fatalf("Display() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestInfoIsReportReadyJSON(t *testing.T) {
	encoded, err := json.Marshal(Info{Version: "v1.2.3", Commit: "abc123", BuildTime: "2026-10-23T20:00:00Z", Dirty: true})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if got, want := string(encoded), `{"version":"v1.2.3","commit":"abc123","build_time":"2026-10-23T20:00:00Z","dirty":true}`; got != want {
		t.Fatalf("JSON = %s, want %s", got, want)
	}
}
