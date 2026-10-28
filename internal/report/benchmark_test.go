package report

import (
	"encoding/json"
	"testing"
)

func BenchmarkReportSerialization(b *testing.B) {
	report := schemaReport(false, true)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if _, err := json.Marshal(report); err != nil {
			b.Fatal(err)
		}
	}
}
