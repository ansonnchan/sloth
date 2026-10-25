// Package render turns domain results into human terminal output.
package render

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/inspect"
)

const (
	ansiCyan  = "\x1b[36m"
	ansiReset = "\x1b[0m"
)

// Options controls presentation without changing measurement semantics.
type Options struct {
	Width int
	Color bool
	ASCII bool
}

// InspectView is the presentation-only shape for one inspection.
type InspectView struct {
	Method         string
	Endpoint       string
	FinalEndpoint  string
	Status         string
	Protocol       string
	Connection     string
	ResponseBytes  string
	Total          string
	Phases         []PhaseView
	Interpretation string
}

// PhaseView is one display-ready timing row.
type PhaseView struct {
	Name  string
	Value string
}

// NewInspectView creates a stable, body-free view model.
func NewInspectView(result inspect.Result) InspectView {
	view := InspectView{
		Method:         result.Request.Method,
		Endpoint:       result.Request.Target.RedactedURL,
		Status:         formatStatus(result.Outcome.StatusCode),
		Protocol:       valueOrNA(result.Protocol),
		Connection:     formatConnection(result.Connection),
		ResponseBytes:  formatBytes(result.Body.TransferredBytes),
		Total:          formatPhase(result.Timings.Total),
		Interpretation: selectInterpretation(result),
		Phases: []PhaseView{
			{Name: "DNS", Value: formatPhase(result.Timings.DNS)},
			{Name: "Connect", Value: formatPhase(result.Timings.Connect)},
			{Name: "TLS", Value: formatPhase(result.Timings.TLS)},
			{Name: "Request write", Value: formatPhase(result.Timings.RequestWrite)},
			{Name: "First-byte wait", Value: formatPhase(result.Timings.FirstByteWait)},
			{Name: "Body transfer", Value: formatPhase(result.Timings.BodyTransfer)},
		},
	}
	if result.Redirects.FinalTarget != nil && result.Redirects.FinalTarget.RedactedURL != view.Endpoint {
		view.FinalEndpoint = result.Redirects.FinalTarget.RedactedURL
	}
	return view
}

// Inspect writes the compact human-readable timing view.
func Inspect(writer io.Writer, view InspectView, options Options) error {
	width := options.Width
	if width < 20 {
		width = 80
	}

	if err := writeTarget(writer, view.Method, view.Endpoint, width, options.Color); err != nil {
		return err
	}
	if view.FinalEndpoint != "" {
		if err := writeField(writer, "Final endpoint", view.FinalEndpoint, width, options.Color, ""); err != nil {
			return err
		}
	}
	for _, field := range []struct{ label, value string }{
		{"Status", view.Status},
		{"Protocol", view.Protocol},
		{"Connection", view.Connection},
		{"Response", view.ResponseBytes},
		{"Total", view.Total},
	} {
		if err := writeField(writer, field.label, field.value, width, options.Color, ""); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, colorLabel("Phases", options.Color)+":"); err != nil {
		return err
	}
	for _, phase := range view.Phases {
		if err := writeField(writer, phase.Name, phase.Value, width, options.Color, "  "); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(writer); err != nil {
		return err
	}
	return writeField(writer, "Interpretation", view.Interpretation, width, options.Color, "")
}

func writeTarget(writer io.Writer, method, endpoint string, width int, color bool) error {
	prefix := method + " "
	if utf8.RuneCountInString(prefix+endpoint) <= width {
		_, err := fmt.Fprintln(writer, colorLabel(method, color)+" "+endpoint)
		return err
	}
	if _, err := fmt.Fprintln(writer, colorLabel(method, color)); err != nil {
		return err
	}
	for _, line := range wrap(endpoint, width-2) {
		if _, err := fmt.Fprintln(writer, "  "+line); err != nil {
			return err
		}
	}
	return nil
}

func writeField(writer io.Writer, label, value string, width int, color bool, indent string) error {
	prefix := indent + label + ": "
	lines := wrap(value, width-utf8.RuneCountInString(prefix))
	if _, err := fmt.Fprintln(writer, indent+colorLabel(label, color)+": "+lines[0]); err != nil {
		return err
	}
	continuation := strings.Repeat(" ", utf8.RuneCountInString(prefix))
	for _, line := range lines[1:] {
		if _, err := fmt.Fprintln(writer, continuation+line); err != nil {
			return err
		}
	}
	return nil
}

func wrap(value string, width int) []string {
	if width < 8 {
		width = 8
	}
	runes := []rune(value)
	if len(runes) == 0 {
		return []string{""}
	}
	lines := make([]string, 0, (len(runes)+width-1)/width)
	for len(runes) > width {
		breakAt := width
		for index := width; index > width/2; index-- {
			if runes[index-1] == ' ' {
				breakAt = index - 1
				break
			}
		}
		lines = append(lines, string(runes[:breakAt]))
		runes = runes[breakAt:]
		for len(runes) > 0 && runes[0] == ' ' {
			runes = runes[1:]
		}
	}
	lines = append(lines, string(runes))
	return lines
}

func colorLabel(label string, enabled bool) string {
	if !enabled {
		return label
	}
	return ansiCyan + label + ansiReset
}

func formatStatus(statusCode int) string {
	if statusCode == 0 {
		return "N/A"
	}
	if text := http.StatusText(statusCode); text != "" {
		return fmt.Sprintf("%d %s", statusCode, text)
	}
	return fmt.Sprintf("%d", statusCode)
}

func formatConnection(connection inspect.Connection) string {
	if !connection.Observed {
		return "N/A"
	}
	if connection.Reused {
		if connection.WasIdle {
			return fmt.Sprintf("reused (idle %.2f ms)", connection.IdleMillis)
		}
		return "reused"
	}
	return "new"
}

func formatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}
	return fmt.Sprintf("%.2f KiB", float64(value)/1024)
}

func formatPhase(phase httpmeasure.PhaseDuration) string {
	value, available := phase.Value()
	if !available {
		return "N/A (" + humanReason(phase.Reason()) + ")"
	}
	return formatDuration(value)
}

func formatDuration(value time.Duration) string {
	if value == 0 {
		return "0 ms"
	}
	if value < time.Millisecond {
		return fmt.Sprintf("%.3f ms", float64(value)/float64(time.Millisecond))
	}
	if value < time.Second {
		return fmt.Sprintf("%.2f ms", float64(value)/float64(time.Millisecond))
	}
	return fmt.Sprintf("%.2f s", value.Seconds())
}

func humanReason(reason httpmeasure.UnavailableReason) string {
	switch reason {
	case httpmeasure.UnavailableMissingStart:
		return "start not observed"
	case httpmeasure.UnavailableMissingEnd:
		return "completion not observed"
	case httpmeasure.UnavailableIncomplete:
		return "incomplete observations"
	case httpmeasure.UnavailableOutOfOrder:
		return "invalid event order"
	case httpmeasure.UnavailableConnectionReuse:
		return "connection reused"
	case httpmeasure.UnavailableCleartext:
		return "cleartext HTTP"
	default:
		return "unavailable"
	}
}

func valueOrNA(value string) string {
	if value == "" {
		return "N/A"
	}
	return value
}

func selectInterpretation(result inspect.Result) string {
	if result.Outcome.Error != nil {
		if result.Outcome.FailurePhase != "" && result.Outcome.FailurePhase != httpmeasure.FailurePhaseUnknown {
			return fmt.Sprintf("The request ended with %s during the client-observed %s phase.", result.Outcome.Category, result.Outcome.FailurePhase)
		}
		return fmt.Sprintf("The request ended with %s before a complete measurement was available.", result.Outcome.Category)
	}

	type candidate struct {
		name  string
		value time.Duration
	}
	candidates := make([]candidate, 0, 6)
	for _, item := range []struct {
		name  string
		phase httpmeasure.PhaseDuration
	}{
		{"DNS lookup", result.Timings.DNS},
		{"connection setup", result.Timings.Connect},
		{"TLS negotiation", result.Timings.TLS},
		{"request write", result.Timings.RequestWrite},
		{"first response byte", result.Timings.FirstByteWait},
		{"body transfer", result.Timings.BodyTransfer},
	} {
		if value, ok := item.phase.Value(); ok {
			candidates = append(candidates, candidate{name: item.name, value: value})
		}
	}
	if len(candidates) == 0 {
		return "The available trace does not identify a dominant client-observed phase."
	}
	largest := candidates[0]
	for _, item := range candidates[1:] {
		if item.value > largest.value {
			largest = item
		}
	}
	if largest.name == "first response byte" {
		return "The longest observed phase was the wait for the first response byte; it can include server work, network, queueing, and upstream delay."
	}
	return fmt.Sprintf("The longest observed phase was %s; investigate it as a client-side observation, not proof of a server-side root cause.", largest.name)
}
