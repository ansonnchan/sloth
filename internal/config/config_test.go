package config

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	got := Defaults()

	if got.Method != "GET" || got.Timeout != 30*time.Second {
		t.Fatalf("request defaults = %s, %s", got.Method, got.Timeout)
	}
	if got.Requests != 100 || got.Warmups != 5 || got.Rate != 5 {
		t.Fatalf("run defaults = %d requests, %d warmups, %g rate", got.Requests, got.Warmups, got.Rate)
	}
	if !reflect.DeepEqual(got.ConcurrencyStages, []int{1, 5, 10}) {
		t.Fatalf("concurrency stages = %v", got.ConcurrencyStages)
	}
	if got.FollowRedirects || got.Color != ColorAuto || got.BodyLimit != 1<<20 || got.IncludeSamples {
		t.Fatalf("policy defaults = %+v", got)
	}
}

func TestOverridesDoNotMutateDefaults(t *testing.T) {
	first := Defaults()
	first.Method = "POST"
	first.ConcurrencyStages[0] = 99

	second := Defaults()
	if second.Method != DefaultMethod {
		t.Fatalf("method default mutated to %q", second.Method)
	}
	if !reflect.DeepEqual(second.ConcurrencyStages, []int{1, 5, 10}) {
		t.Fatalf("concurrency defaults mutated to %v", second.ConcurrencyStages)
	}
}

func TestEffectiveCopiesMutableInputAndOutput(t *testing.T) {
	options := Defaults()
	effective := Resolve(options)

	options.ConcurrencyStages[0] = 99
	got := effective.ConcurrencyStages()
	got[1] = 99

	if !reflect.DeepEqual(effective.ConcurrencyStages(), []int{1, 5, 10}) {
		t.Fatalf("effective stages mutated to %v", effective.ConcurrencyStages())
	}
}

func TestEffectiveJSONUsesSafeExplicitUnits(t *testing.T) {
	encoded, err := json.Marshal(Resolve(Defaults()))
	if err != nil {
		t.Fatalf("marshal effective config: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("unmarshal effective config: %v", err)
	}

	if got["timeout_ms"] != float64(30_000) {
		t.Fatalf("timeout_ms = %v", got["timeout_ms"])
	}
	for _, forbidden := range []string{"headers", "url", "body", "authorization", "cookie"} {
		if _, exists := got[forbidden]; exists {
			t.Fatalf("serialized config contains forbidden field %q", forbidden)
		}
	}
}
