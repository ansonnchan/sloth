package compare

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/ansonnchan/sloth/internal/report"
)

func FuzzCompatibilityAndDeltas(f *testing.F) {
	seed, err := json.Marshal(compatibleReport())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"schema_version":"sloth.report/v1.0","experiments":[]}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			t.Skip()
		}
		var candidate report.Report
		if err := json.Unmarshal(data, &candidate); err != nil {
			return
		}
		compatibility := Check(compatibleReport(), candidate, ModeDescriptive)
		if compatibility.DescriptiveEligible {
			_, _ = Calculate(compatibility)
		}
	})
}

func FuzzDeltaArithmetic(f *testing.F) {
	f.Add(0.0, 1.0, true, true)
	f.Add(100.0, 120.0, true, true)
	f.Add(1.0, 0.0, false, true)
	f.Fuzz(func(t *testing.T, before, after float64, beforeAvailable, afterAvailable bool) {
		if math.IsNaN(before) || math.IsNaN(after) || math.IsInf(before, 0) || math.IsInf(after, 0) {
			return
		}
		var beforeValue, afterValue *float64
		if beforeAvailable {
			beforeValue = &before
		}
		if afterAvailable {
			afterValue = &after
		}
		delta := newDelta("fuzz", "ms", LowerIsBetter, beforeValue, afterValue)
		if beforeValue == nil || afterValue == nil {
			if delta.Absolute != nil || delta.PercentChange != nil {
				t.Fatalf("unavailable input became numeric: %+v", delta)
			}
		}
		if beforeValue != nil && *beforeValue == 0 && delta.PercentChange != nil {
			t.Fatalf("zero baseline became a percentage: %+v", delta)
		}
	})
}
