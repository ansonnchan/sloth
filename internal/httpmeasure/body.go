package httpmeasure

import (
	"errors"
	"fmt"
	"io"
	"math"
)

// BodyReadResult describes bounded response-body consumption without retaining
// any body content.
type BodyReadResult struct {
	TransferredBytes int64 `json:"transferred_bytes"`
	LimitBytes       int64 `json:"limit_bytes"`
	Complete         bool  `json:"complete"`
	Truncated        bool  `json:"truncated"`
	ReuseEligible    bool  `json:"reuse_eligible"`
}

// DrainBody streams a response body to io.Discard, probes at most one byte past
// the configured cap to detect truncation, and closes the body on every path.
func DrainBody(body io.ReadCloser, limitBytes int64) (BodyReadResult, error) {
	result := BodyReadResult{LimitBytes: limitBytes}
	if body == nil {
		return result, errors.New("read response body: body is required")
	}
	if limitBytes <= 0 {
		closeErr := body.Close()
		if closeErr != nil {
			return result, errors.Join(errors.New("read response body: limit must be greater than zero"), fmt.Errorf("close response body: %w", closeErr))
		}
		return result, errors.New("read response body: limit must be greater than zero")
	}

	readLimit := limitBytes
	if limitBytes < math.MaxInt64 {
		readLimit++
	}
	var readErr error
	result.TransferredBytes, readErr = io.Copy(io.Discard, io.LimitReader(body, readLimit))
	closeErr := body.Close()

	if result.TransferredBytes > limitBytes {
		result.Truncated = true
		return result, joinBodyErrors(NewBodyLimitError(limitBytes, result.TransferredBytes), closeErr)
	}
	if readErr != nil {
		return result, joinBodyErrors(fmt.Errorf("read response body: %w", readErr), closeErr)
	}
	if closeErr != nil {
		return result, fmt.Errorf("close response body: %w", closeErr)
	}

	result.Complete = true
	result.ReuseEligible = true
	return result, nil
}

func joinBodyErrors(primary, closeErr error) error {
	if closeErr == nil {
		return primary
	}
	return errors.Join(primary, fmt.Errorf("close response body: %w", closeErr))
}
