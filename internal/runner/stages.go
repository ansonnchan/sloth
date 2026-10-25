package runner

import (
	"errors"

	"github.com/ansonnchan/sloth/internal/config"
)

var defaultConcurrencySteps = []int{1, 5, 10}

// GenerateConcurrencyStages returns ascending, unique stages that always
// include one and the requested maximum without crossing the absolute ceiling.
// Default landmarks below the maximum remain useful comparison points.
func GenerateConcurrencyStages(maximum int) ([]int, error) {
	if maximum <= 0 {
		return nil, errors.New("generate concurrency stages: maximum must be greater than zero")
	}
	if maximum > config.AbsoluteConcurrency {
		return nil, errors.New("generate concurrency stages: maximum exceeds the absolute concurrency ceiling")
	}
	stages := make([]int, 0, len(defaultConcurrencySteps)+1)
	for _, candidate := range defaultConcurrencySteps {
		if candidate <= maximum {
			stages = append(stages, candidate)
		}
	}
	if stages[len(stages)-1] != maximum {
		stages = append(stages, maximum)
	}
	return stages, nil
}
