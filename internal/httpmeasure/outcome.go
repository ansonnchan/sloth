package httpmeasure

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
)

// OutcomeCategory is the stable public classification of one request attempt.
type OutcomeCategory string

const (
	OutcomeSuccess        OutcomeCategory = "success"
	OutcomeHTTPError      OutcomeCategory = "http_error"
	OutcomeTimeout        OutcomeCategory = "timeout"
	OutcomeTransportError OutcomeCategory = "transport_error"
	OutcomeCancelled      OutcomeCategory = "cancelled"
	OutcomeBodyLimit      OutcomeCategory = "body_limit"
)

// FailurePhase identifies where an unsuccessful request was observed. It does
// not claim a server-side cause.
type FailurePhase string

const (
	FailurePhaseUnknown FailurePhase = "unknown"
	FailurePhaseDNS     FailurePhase = "dns"
	FailurePhaseConnect FailurePhase = "connect"
	FailurePhaseTLS     FailurePhase = "tls"
	FailurePhaseRequest FailurePhase = "request"
	FailurePhaseBody    FailurePhase = "response_body"
)

// Outcome preserves response evidence even when the attempt is incomplete.
type Outcome struct {
	Category         OutcomeCategory `json:"category"`
	FailurePhase     FailurePhase    `json:"failure_phase,omitempty"`
	StatusCode       int             `json:"status_code,omitempty"`
	ResponseReceived bool            `json:"response_received"`
	Complete         bool            `json:"complete"`
	TransferredBytes int64           `json:"transferred_bytes"`
	Error            *RequestError   `json:"error,omitempty"`
}

// RequestError retains the original Go error for errors.Is/errors.As while
// exposing only a stable, redacted classification to users and serializers.
type RequestError struct {
	category OutcomeCategory
	phase    FailurePhase
	message  string
	cause    error
}

// BodyLimitError reports that more response bytes were available than policy
// allowed Sloth to read.
type BodyLimitError struct {
	LimitBytes       int64
	TransferredBytes int64
}

func (e *BodyLimitError) Error() string {
	return fmt.Sprintf("response body exceeded the %d-byte limit after %d bytes", e.LimitBytes, e.TransferredBytes)
}

// NewBodyLimitError constructs the typed error used by bounded body readers.
func NewBodyLimitError(limitBytes, transferredBytes int64) error {
	return &BodyLimitError{LimitBytes: limitBytes, TransferredBytes: transferredBytes}
}

// ClassifyOutcome maps a completed response or terminal error into the stable
// public taxonomy. A nonzero status and byte count remain available on failure.
func ClassifyOutcome(statusCode int, transferredBytes int64, terminalErr error) Outcome {
	outcome := Outcome{
		StatusCode:       statusCode,
		ResponseReceived: statusCode != 0,
		Complete:         terminalErr == nil,
		TransferredBytes: transferredBytes,
	}

	if terminalErr == nil {
		if statusCode >= 400 {
			outcome.Category = OutcomeHTTPError
		} else {
			outcome.Category = OutcomeSuccess
		}
		return outcome
	}

	requestErr := classifyRequestError(terminalErr)
	outcome.Category = requestErr.category
	outcome.FailurePhase = requestErr.phase
	outcome.Error = requestErr
	return outcome
}

// Category returns the stable public error category.
func (e *RequestError) Category() OutcomeCategory { return e.category }

// Phase returns the client-observable failure phase.
func (e *RequestError) Phase() FailurePhase { return e.phase }

func (e *RequestError) Error() string { return e.message }

func (e *RequestError) Unwrap() error { return e.cause }

// Format prevents Go-syntax formatting from traversing the private cause.
func (e *RequestError) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, e.message)
}

// MarshalJSON emits only the stable public error representation.
func (e *RequestError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Category OutcomeCategory `json:"category"`
		Phase    FailurePhase    `json:"phase"`
		Message  string          `json:"message"`
	}{
		Category: e.category,
		Phase:    e.phase,
		Message:  e.message,
	})
}

func classifyRequestError(err error) *RequestError {
	phase := classifyFailurePhase(err)
	category := OutcomeTransportError
	message := phaseMessage(phase)

	var bodyLimitErr *BodyLimitError
	switch {
	case errors.As(err, &bodyLimitErr):
		category = OutcomeBodyLimit
		phase = FailurePhaseBody
		message = bodyLimitErr.Error()
	case errors.Is(err, context.Canceled):
		category = OutcomeCancelled
		message = "request was cancelled"
	case errors.Is(err, context.DeadlineExceeded) || isTimeout(err):
		category = OutcomeTimeout
		message = "request timed out"
	}

	return &RequestError{
		category: category,
		phase:    phase,
		message:  message,
		cause:    err,
	}
}

func classifyFailurePhase(err error) FailurePhase {
	var bodyLimitErr *BodyLimitError
	if errors.As(err, &bodyLimitErr) || errors.Is(err, io.ErrUnexpectedEOF) {
		return FailurePhaseBody
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return FailurePhaseDNS
	}

	var certificateVerificationErr *tls.CertificateVerificationError
	var recordHeaderErr tls.RecordHeaderError
	var unknownAuthorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var certificateInvalidErr x509.CertificateInvalidError
	if errors.As(err, &certificateVerificationErr) ||
		errors.As(err, &recordHeaderErr) ||
		errors.As(err, &unknownAuthorityErr) ||
		errors.As(err, &hostnameErr) ||
		errors.As(err, &certificateInvalidErr) {
		return FailurePhaseTLS
	}

	var operationErr *net.OpError
	if errors.As(err, &operationErr) {
		switch strings.ToLower(operationErr.Op) {
		case "dial", "connect":
			return FailurePhaseConnect
		}
	}

	return FailurePhaseUnknown
}

func isTimeout(err error) bool {
	var networkErr net.Error
	return errors.As(err, &networkErr) && networkErr.Timeout()
}

func phaseMessage(phase FailurePhase) string {
	switch phase {
	case FailurePhaseDNS:
		return "DNS lookup failed"
	case FailurePhaseConnect:
		return "connection failed"
	case FailurePhaseTLS:
		return "TLS negotiation failed"
	case FailurePhaseBody:
		return "response body transfer failed"
	default:
		return "HTTP transport failed"
	}
}
