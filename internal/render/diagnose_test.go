package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/diagnosis"
)

func TestDiagnoseSuccessSnapshot(t *testing.T) {
	document := diagnose.Document{
		RuleVersion: "1.0.0",
		Plan:        diagnose.PlanSummary{MeasuredRequests: 600, WarmupRequests: 30, TotalRequests: 630},
		Experiments: []diagnose.ExperimentSummary{
			{Name: "baseline", Samples: 100, P50Millis: number(40), P95Millis: number(55), P99Millis: number(60), OfferedPerSecond: 5, AchievedPerSecond: 5, ActualConcurrency: 0.4},
			{Name: "concurrency-10", Samples: 100, P50Millis: number(42), P95Millis: number(60), P99Millis: number(70), OfferedPerSecond: 5, AchievedPerSecond: 5, ActualConcurrency: 0.5},
		},
		Findings: []diagnosis.Finding{},
	}
	var output bytes.Buffer
	if err := Diagnose(&output, document, Options{Width: 120, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	want := "Sloth diagnosis\n" +
		"Rules: 1.0.0\n" +
		"Plan: 600 measured + 30 warm-up requests (630 total)\n\n" +
		"Experiment summary:\n" +
		"  Experiment          N      p50      p95      p99  Err  Offered  Achieved  Actual\n" +
		"  baseline            100   40.0ms   55.0ms   60.0ms    0    5.00/s     5.00/s    0.40\n" +
		"  concurrency-10      100   42.0ms   60.0ms   70.0ms    0    5.00/s     5.00/s    0.50\n\n" +
		"Findings:\n" +
		"  No diagnostic thresholds were crossed with the available evidence.\n\n" +
		"Next steps:\n" +
		"  - Keep this workload unchanged when comparing a future run.\n"
	if output.String() != want {
		t.Fatalf("snapshot:\n%s\nwant:\n%s", output.String(), want)
	}
}

func TestDiagnoseMixedFindingsSnapshotSections(t *testing.T) {
	document := findingDocument(diagnosis.Finding{
		Code: diagnosis.CodeHighFirstByteWait, Title: "First byte dominated latency",
		Confidence: diagnosis.ConfidenceMedium, Observation: "First-byte wait was 75% of median latency.",
		PossibleCauses: []string{"request processing", "dependency latency"},
		Recommendation: "Correlate with server timing.",
		Evidence:       diagnosis.Evidence{Metrics: []diagnosis.Metric{{Name: "first_byte_share", Value: .75, Unit: diagnosis.UnitRatio}}},
	})
	var output bytes.Buffer
	if err := Diagnose(&output, document, Options{Width: 100, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"HIGH_FIRST_BYTE_WAIT - First byte dominated latency", "Observation:", "Possible causes:",
		"Recommendation:", "Confidence: medium (evidence quality, not probability)", "Severity:", "Evidence:", "Next steps:",
	} {
		if !strings.Contains(output.String(), required) {
			t.Fatalf("output lacks %q:\n%s", required, output.String())
		}
	}
}

func TestDiagnoseInsufficientEvidenceHasLimitationsAndNoCauseClaim(t *testing.T) {
	document := findingDocument(diagnosis.Finding{
		Code: diagnosis.CodeInsufficientEvidence, Title: "Inconclusive", Confidence: diagnosis.ConfidenceLow,
		Observation: "Diagnosis was inconclusive.", Recommendation: "Repeat the bounded suite.",
		Evidence: diagnosis.Evidence{Notes: []string{"only 3 samples are available"}},
	})
	document.Limitations = []string{"only 3 samples are available"}
	var output bytes.Buffer
	if err := Diagnose(&output, document, Options{Width: 80, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Limitations:") || strings.Contains(output.String(), "Possible causes:") {
		t.Fatalf("inconclusive output overclaims or omits limitations:\n%s", output.String())
	}
}

func TestDiagnoseNarrowASCIIIsStaticAndReadable(t *testing.T) {
	document := findingDocument(diagnosis.Finding{
		Code: diagnosis.CodeConcurrencySensitivity, Title: "Concurrency changed observed behavior",
		Confidence:     diagnosis.ConfidenceLow,
		Observation:    "A deliberately long observation wraps without cursor movement or animation.",
		Recommendation: "Repeat at an intermediate achieved rate.",
	})
	var output bytes.Buffer
	if err := Diagnose(&output, document, Options{Width: 40, ASCII: true}); err != nil {
		t.Fatal(err)
	}
	text := output.String()
	for _, required := range []string{"samples=100", "actual concurrency=", "Observation:", "Next steps:"} {
		if !strings.Contains(text, required) {
			t.Fatalf("narrow output lacks %q:\n%s", required, text)
		}
	}
	if strings.Contains(text, "\x1b[") || strings.Contains(text, "\r") {
		t.Fatalf("narrow output contains terminal controls: %q", text)
	}
}

func findingDocument(finding diagnosis.Finding) diagnose.Document {
	return diagnose.Document{
		RuleVersion: "1.0.0", Plan: diagnose.PlanSummary{MeasuredRequests: 100, TotalRequests: 100},
		Experiments: []diagnose.ExperimentSummary{{
			Name: "baseline", Samples: 100, P50Millis: number(100), P95Millis: number(200), P99Millis: number(250),
			OfferedPerSecond: 5, AchievedPerSecond: 4.9, ActualConcurrency: .8,
		}},
		Findings: []diagnosis.Finding{finding},
	}
}

func number(value float64) *float64 { return &value }
