package compare

import (
	"fmt"
	"math"
)

type GateMetric string

const (
	GateP50 GateMetric = "p50"
	GateP95 GateMetric = "p95"
	GateP99 GateMetric = "p99"
)

func (metric GateMetric) Valid() bool {
	return metric == GateP50 || metric == GateP95 || metric == GateP99
}

type Regression struct {
	ExperimentID string  `json:"experiment_id"`
	Metric       string  `json:"metric"`
	Before       float64 `json:"before"`
	After        float64 `json:"after"`
	Percent      float64 `json:"percent"`
}

type GateResult struct {
	Metric      GateMetric   `json:"metric"`
	Threshold   float64      `json:"threshold_percent"`
	Regressions []Regression `json:"regressions"`
}

// EvaluateGate applies the threshold to raw, unrounded latency percentages for
// every compatible experiment. Missing values and zero baselines are inadequate
// evidence rather than automatic passes.
func EvaluateGate(deltas DeltaReport, metric GateMetric, threshold float64) (GateResult, error) {
	if !metric.Valid() {
		return GateResult{}, fmt.Errorf("gate metric must be p50, p95, or p99, got %q", metric)
	}
	if threshold < 0 || math.IsNaN(threshold) || math.IsInf(threshold, 0) {
		return GateResult{}, fmt.Errorf("regression threshold must be a finite non-negative percentage")
	}
	result := GateResult{Metric: metric, Threshold: threshold, Regressions: []Regression{}}
	metricName := "latency." + string(metric) + "_ms"
	if len(deltas.Experiments) == 0 {
		return GateResult{}, fmt.Errorf("gate has no compatible experiments")
	}
	for _, experiment := range deltas.Experiments {
		var selected *MetricDelta
		for index := range experiment.Metrics {
			if experiment.Metrics[index].Name == metricName {
				selected = &experiment.Metrics[index]
				break
			}
		}
		if selected == nil || selected.Before == nil || selected.After == nil || selected.PercentChange == nil {
			return GateResult{}, fmt.Errorf("gate metric %s is unavailable or has a zero baseline for experiment %q", metric, experiment.ID)
		}
		if *selected.PercentChange > threshold {
			result.Regressions = append(result.Regressions, Regression{
				ExperimentID: experiment.ID, Metric: metricName,
				Before: *selected.Before, After: *selected.After, Percent: *selected.PercentChange,
			})
		}
	}
	return result, nil
}
