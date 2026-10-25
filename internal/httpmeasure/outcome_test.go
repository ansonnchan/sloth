package httpmeasure

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"testing"
)

func TestClassifyOutcome(t *testing.T) {
	dnsErr := &net.DNSError{Name: "private.example", Err: "no such host"}
	cases := []struct {
		name      string
		status    int
		bytes     int64
		err       error
		category  OutcomeCategory
		phase     FailurePhase
		complete  bool
		responded bool
	}{
		{name: "success", status: 204, category: OutcomeSuccess, complete: true, responded: true},
		{name: "redirect", status: 302, category: OutcomeSuccess, complete: true, responded: true},
		{name: "client HTTP error", status: 404, bytes: 12, category: OutcomeHTTPError, complete: true, responded: true},
		{name: "server HTTP error", status: 503, bytes: 9, category: OutcomeHTTPError, complete: true, responded: true},
		{name: "deadline", err: fmt.Errorf("wrapped: %w", context.DeadlineExceeded), category: OutcomeTimeout, phase: FailurePhaseUnknown},
		{name: "cancelled", err: fmt.Errorf("wrapped: %w", context.Canceled), category: OutcomeCancelled, phase: FailurePhaseUnknown},
		{name: "DNS", err: &url.Error{Op: "Get", URL: "https://private.example", Err: dnsErr}, category: OutcomeTransportError, phase: FailurePhaseDNS},
		{name: "connect", err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}, category: OutcomeTransportError, phase: FailurePhaseConnect},
		{name: "TLS", err: x509.UnknownAuthorityError{}, category: OutcomeTransportError, phase: FailurePhaseTLS},
		{name: "body", status: 200, bytes: 6, err: io.ErrUnexpectedEOF, category: OutcomeTransportError, phase: FailurePhaseBody, responded: true},
		{name: "body limit", status: 200, bytes: 11, err: NewBodyLimitError(10, 11), category: OutcomeBodyLimit, phase: FailurePhaseBody, responded: true},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			outcome := ClassifyOutcome(test.status, test.bytes, test.err)
			if outcome.Category != test.category {
				t.Fatalf("Category = %q, want %q", outcome.Category, test.category)
			}
			if outcome.FailurePhase != test.phase {
				t.Fatalf("FailurePhase = %q, want %q", outcome.FailurePhase, test.phase)
			}
			if outcome.Complete != test.complete {
				t.Fatalf("Complete = %v, want %v", outcome.Complete, test.complete)
			}
			if outcome.ResponseReceived != test.responded {
				t.Fatalf("ResponseReceived = %v, want %v", outcome.ResponseReceived, test.responded)
			}
			if outcome.StatusCode != test.status || outcome.TransferredBytes != test.bytes {
				t.Fatalf("partial evidence = (%d, %d), want (%d, %d)", outcome.StatusCode, outcome.TransferredBytes, test.status, test.bytes)
			}
		})
	}
}

func TestTimeoutRetainsObservedPhase(t *testing.T) {
	dnsTimeout := &net.DNSError{Name: "example.com", Err: "timeout", IsTimeout: true}
	outcome := ClassifyOutcome(0, 0, dnsTimeout)

	if outcome.Category != OutcomeTimeout || outcome.FailurePhase != FailurePhaseDNS {
		t.Fatalf("outcome = (%q, %q), want timeout during DNS", outcome.Category, outcome.FailurePhase)
	}
}

func TestRequestErrorKeepsCauseButFormatsSafely(t *testing.T) {
	const secret = "url-secret"
	cause := &url.Error{
		Op:  "Get",
		URL: "https://example.com/path?token=" + secret,
		Err: &net.DNSError{Name: "example.com", Err: "no such host"},
	}
	outcome := ClassifyOutcome(0, 0, fmt.Errorf("request failed: %w", cause))

	if !errors.Is(outcome.Error, cause) {
		t.Fatal("public request error does not retain its internal cause")
	}
	var dnsErr *net.DNSError
	if !errors.As(outcome.Error, &dnsErr) {
		t.Fatal("public request error does not support errors.As for its cause")
	}

	encoded, err := json.Marshal(outcome)
	if err != nil {
		t.Fatalf("json.Marshal(outcome) error = %v", err)
	}
	formatted := strings.Join([]string{
		outcome.Error.Error(),
		fmt.Sprintf("%v", outcome.Error),
		fmt.Sprintf("%+v", outcome.Error),
		fmt.Sprintf("%#v", outcome.Error),
		string(encoded),
	}, "\n")
	if strings.Contains(formatted, secret) {
		t.Fatalf("safe output contains secret: %s", formatted)
	}
	if !strings.Contains(formatted, "DNS lookup failed") {
		t.Fatalf("safe output lacks stable message: %s", formatted)
	}
}

func TestBodyLimitErrorIsTyped(t *testing.T) {
	err := fmt.Errorf("read response: %w", NewBodyLimitError(10, 11))
	var bodyLimitErr *BodyLimitError
	if !errors.As(err, &bodyLimitErr) {
		t.Fatal("errors.As did not find BodyLimitError")
	}
	if bodyLimitErr.LimitBytes != 10 || bodyLimitErr.TransferredBytes != 11 {
		t.Fatalf("BodyLimitError = %#v", bodyLimitErr)
	}
}
