package compare

import "testing"

func TestEvaluateGateUsesRawConfiguredMetric(t *testing.T) {
	deltas := DeltaReport{Experiments: []ExperimentDelta{{
		ID: "baseline",
		Metrics: []MetricDelta{
			{Name: "latency.p50_ms", Before: floatPointer(10), After: floatPointer(30), PercentChange: floatPointer(200)},
			{Name: "latency.p95_ms", Before: floatPointer(100), After: floatPointer(120.0001), PercentChange: floatPointer(20.0001)},
		},
	}}}
	result, err := EvaluateGate(deltas, GateP95, 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Regressions) != 1 || result.Regressions[0].Percent != 20.0001 {
		t.Fatalf("result = %+v", result)
	}
}

func TestEvaluateGateAllowsThresholdBoundaryAndImprovement(t *testing.T) {
	deltas := DeltaReport{Experiments: []ExperimentDelta{
		{ID: "boundary", Metrics: []MetricDelta{{Name: "latency.p95_ms", Before: floatPointer(100), After: floatPointer(120), PercentChange: floatPointer(20)}}},
		{ID: "improved", Metrics: []MetricDelta{{Name: "latency.p95_ms", Before: floatPointer(100), After: floatPointer(80), PercentChange: floatPointer(-20)}}},
	}}
	result, err := EvaluateGate(deltas, GateP95, 20)
	if err != nil || len(result.Regressions) != 0 {
		t.Fatalf("result = %+v, error = %v", result, err)
	}
}

func TestEvaluateGateRejectsInadequateEvidenceAndInvalidOptions(t *testing.T) {
	tests := []struct {
		name      string
		deltas    DeltaReport
		metric    GateMetric
		threshold float64
	}{
		{"no experiments", DeltaReport{}, GateP95, 20},
		{"missing metric", DeltaReport{Experiments: []ExperimentDelta{{ID: "baseline"}}}, GateP95, 20},
		{"zero baseline", DeltaReport{Experiments: []ExperimentDelta{{ID: "baseline", Metrics: []MetricDelta{{Name: "latency.p95_ms", Before: floatPointer(0), After: floatPointer(1)}}}}}, GateP95, 20},
		{"invalid metric", DeltaReport{}, GateMetric("mean"), 20},
		{"negative threshold", DeltaReport{}, GateP95, -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := EvaluateGate(test.deltas, test.metric, test.threshold); err == nil {
				t.Fatal("EvaluateGate() succeeded")
			}
		})
	}
}
