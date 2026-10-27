package render

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/compare"
	"github.com/ansonnchan/sloth/internal/experiment"
)

func TestComparisonSnapshots(t *testing.T) {
	tests := []struct {
		name          string
		compatibility compare.Compatibility
		metrics       []compare.MetricDelta
		want          []string
	}{
		{
			name: "improvement", compatibility: exactCompatibility(),
			metrics: []compare.MetricDelta{deltaView("latency.p95_ms", "ms", compare.LowerIsBetter, 100, 80, -20, -20)},
			want:    []string{"Compatibility: exact", "100 ms", "80 ms", "-20 ms", "-20.00%", "improved", "Observed improvements"},
		},
		{
			name: "regression", compatibility: exactCompatibility(),
			metrics: []compare.MetricDelta{deltaView("latency.p95_ms", "ms", compare.LowerIsBetter, 100, 125, 25, 25)},
			want:    []string{"+25 ms", "+25.00%", "regressed", "Observed regressions"},
		},
		{
			name: "mixed", compatibility: exactCompatibility(),
			metrics: []compare.MetricDelta{
				deltaView("latency.p95_ms", "ms", compare.LowerIsBetter, 100, 80, -20, -20),
				deltaView("outcomes.error_rate_percent", "%", compare.LowerIsBetter, 1, 2, 1, 100),
			},
			want: []string{"improved", "regressed", "Mixed observed changes"},
		},
		{
			name: "zero baseline", compatibility: exactCompatibility(),
			metrics: []compare.MetricDelta{{Name: "throughput.completed_per_second", Unit: "requests/s", Direction: compare.HigherIsBetter, Before: ptr(0), After: ptr(5), Absolute: ptr(5)}},
			want:    []string{"0/s", "5/s", "+5/s", "N/A", "zero or unavailable baseline", "improved"},
		},
		{
			name:          "incompatible",
			compatibility: compare.Compatibility{Mode: compare.ModeDescriptive, Fatal: []compare.Mismatch{{Field: "target.fingerprint", Before: "a", After: "b", Message: "target identities differ"}}, Pairs: []compare.ExperimentPair{}},
			want:          []string{"Compatibility warnings:", "target.fingerprint", "target identities differ", "Reports are not comparable", "no deltas were calculated"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			deltas := compare.DeltaReport{}
			if test.compatibility.DescriptiveEligible {
				deltas.Experiments = []compare.ExperimentDelta{{ID: "baseline", Kind: experiment.KindBaseline, Metrics: test.metrics}}
			}
			if err := Comparison(&output, test.compatibility, deltas, Options{Width: 120, ASCII: true}); err != nil {
				t.Fatal(err)
			}
			for _, want := range test.want {
				if !strings.Contains(output.String(), want) {
					t.Fatalf("snapshot lacks %q:\n%s", want, output.String())
				}
			}
		})
	}
}

func TestComparisonShowsWarningsBeforeMetricsAndConclusion(t *testing.T) {
	compatibility := exactCompatibility()
	compatibility.Warnings = []compare.Mismatch{{Field: "configuration.timeout_ms", Before: "1000", After: "2000", Message: "request timeouts differ"}}
	compatibility.GateEligible = false
	deltas := compare.DeltaReport{Experiments: []compare.ExperimentDelta{{
		ID: "baseline", Kind: experiment.KindBaseline,
		Metrics: []compare.MetricDelta{deltaView("latency.p50_ms", "ms", compare.LowerIsBetter, 10, 11, 1, 10)},
	}}}
	var output bytes.Buffer
	if err := Comparison(&output, compatibility, deltas, Options{Width: 60, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	warningIndex := strings.Index(text, "Compatibility warnings")
	metricIndex := strings.Index(text, "latency.p50_ms")
	conclusionIndex := strings.Index(text, "Conclusion:")
	if warningIndex < 0 || metricIndex <= warningIndex || conclusionIndex <= metricIndex || !strings.Contains(text, "result=regressed") {
		t.Fatalf("ordering or narrow result is wrong:\n%s", text)
	}
}

func exactCompatibility() compare.Compatibility {
	return compare.Compatibility{Mode: compare.ModeDescriptive, DescriptiveEligible: true, GateEligible: true, Fatal: []compare.Mismatch{}, Warnings: []compare.Mismatch{}}
}

func deltaView(name, unit string, direction compare.Direction, before, after, absolute, percent float64) compare.MetricDelta {
	return compare.MetricDelta{Name: name, Unit: unit, Direction: direction, Before: ptr(before), After: ptr(after), Absolute: ptr(absolute), PercentChange: ptr(percent)}
}

func ptr(value float64) *float64 { return &value }

func ExampleComparison() {
	compatibility := exactCompatibility()
	deltas := compare.DeltaReport{Experiments: []compare.ExperimentDelta{{
		ID: "baseline", Kind: experiment.KindBaseline,
		Metrics: []compare.MetricDelta{deltaView("latency.p95_ms", "ms", compare.LowerIsBetter, 100, 80, -20, -20)},
	}}}
	var output bytes.Buffer
	_ = Comparison(&output, compatibility, deltas, Options{Width: 120})
	fmt.Println(strings.Contains(output.String(), "improved"))
	// Output: true
}
