package render

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ansonnchan/sloth/internal/compare"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/experiment"
)

func TestRepresentativeWidthsRemainStaticAndReadable(t *testing.T) {
	tests := []struct {
		name  string
		width int
		run   func(*bytes.Buffer) error
		want  []string
	}{
		{
			name: "inspect-40", width: 40,
			run: func(output *bytes.Buffer) error {
				return Inspect(output, InspectView{
					Method: "GET", Endpoint: "https://example.com/a/deliberately/long/redacted/path", Status: "200 OK",
					Protocol: "HTTP/1.1", Connection: "reused", ResponseBytes: "100 B", Total: "10 ms",
					Phases: []PhaseView{{Name: "First-byte wait", Value: "8 ms"}}, Interpretation: "The request completed with bounded evidence.",
				}, Options{Width: 40, ASCII: true})
			},
			want: []string{"GET", "Phases:", "Interpretation:"},
		},
		{
			name: "diagnose-60", width: 60,
			run: func(output *bytes.Buffer) error {
				return Diagnose(output, diagnose.Document{
					RuleVersion: "1.0.0", Plan: diagnose.PlanSummary{MeasuredRequests: 10, TotalRequests: 10},
					Experiments: []diagnose.ExperimentSummary{{Name: "baseline", Samples: 10, P50Millis: number(10), P95Millis: number(12), P99Millis: number(14)}},
				}, Options{Width: 60, ASCII: true})
			},
			want: []string{"Experiment summary:", "samples=10", "Next steps:"},
		},
		{
			name: "compare-80", width: 80,
			run: func(output *bytes.Buffer) error {
				return Comparison(output,
					compare.Compatibility{Mode: compare.ModeDescriptive, DescriptiveEligible: true, GateEligible: true},
					compare.DeltaReport{Experiments: []compare.ExperimentDelta{{
						ID: "baseline", Kind: experiment.KindBaseline,
						Metrics: []compare.MetricDelta{{Name: "latency.p95_ms", Unit: "ms", Direction: compare.LowerIsBetter, Before: number(10), After: number(12), Absolute: number(2), PercentChange: number(20)}},
					}}}, Options{Width: 80, ASCII: true})
			},
			want: []string{"Compatibility:", "result=regressed", "Conclusion:"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := test.run(&output); err != nil {
				t.Fatal(err)
			}
			text := output.String()
			for _, want := range test.want {
				if !strings.Contains(text, want) {
					t.Fatalf("output lacks %q:\n%s", want, text)
				}
			}
			if strings.Contains(text, "\x1b[") || strings.Contains(text, "\r") || strings.ContainsAny(text, "✓✗⚠—") {
				t.Fatalf("noninteractive ASCII output contains terminal-only tokens: %q", text)
			}
			for _, line := range strings.Split(strings.TrimSuffix(text, "\n"), "\n") {
				if utf8.RuneCountInString(line) > test.width+8 {
					t.Fatalf("line is unreadably wider than %d columns: %q", test.width, line)
				}
			}
		})
	}
}
