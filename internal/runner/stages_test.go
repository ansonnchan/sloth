package runner

import (
	"slices"
	"strconv"
	"testing"

	"github.com/ansonnchan/sloth/internal/config"
)

func TestGenerateConcurrencyStages(t *testing.T) {
	for _, test := range []struct {
		maximum int
		want    []int
	}{
		{maximum: 1, want: []int{1}},
		{maximum: 3, want: []int{1, 3}},
		{maximum: 5, want: []int{1, 5}},
		{maximum: 7, want: []int{1, 5, 7}},
		{maximum: 10, want: []int{1, 5, 10}},
		{maximum: 15, want: []int{1, 5, 10, 15}},
		{maximum: config.AbsoluteConcurrency, want: []int{1, 5, 10, config.AbsoluteConcurrency}},
	} {
		t.Run(strconv.Itoa(test.maximum), func(t *testing.T) {
			got, err := GenerateConcurrencyStages(test.maximum)
			if err != nil {
				t.Fatalf("GenerateConcurrencyStages(%d) error = %v", test.maximum, err)
			}
			if !slices.Equal(got, test.want) {
				t.Fatalf("GenerateConcurrencyStages(%d) = %v, want %v", test.maximum, got, test.want)
			}
			for index, stage := range got {
				if stage <= 0 || stage > test.maximum || index > 0 && stage <= got[index-1] {
					t.Fatalf("invalid stages: %v", got)
				}
			}
		})
	}
}

func TestGenerateConcurrencyStagesRejectsInvalidMaximum(t *testing.T) {
	for _, maximum := range []int{0, -1, config.AbsoluteConcurrency + 1} {
		if _, err := GenerateConcurrencyStages(maximum); err == nil {
			t.Fatalf("GenerateConcurrencyStages(%d) error = nil", maximum)
		}
	}
}
