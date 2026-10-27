// Package compare evaluates whether persisted report evidence can be compared.
package compare

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/ansonnchan/sloth/internal/report"
)

type Mode string

const (
	ModeDescriptive Mode = "descriptive"
	ModeGate        Mode = "gate"
)

type Mismatch struct {
	Field        string `json:"field"`
	Before       string `json:"before"`
	After        string `json:"after"`
	ExperimentID string `json:"experiment_id,omitempty"`
	Message      string `json:"message"`
}

type ExperimentPair struct {
	ID     string            `json:"id"`
	Before report.Experiment `json:"before"`
	After  report.Experiment `json:"after"`
}

type Compatibility struct {
	Mode                Mode             `json:"mode"`
	DescriptiveEligible bool             `json:"descriptive_eligible"`
	GateEligible        bool             `json:"gate_eligible"`
	Fatal               []Mismatch       `json:"fatal"`
	Warnings            []Mismatch       `json:"warnings"`
	Pairs               []ExperimentPair `json:"pairs"`
}

// Check evaluates global report identity and then pairs experiments by stable
// ID. Gate mode promotes every material mismatch to fatal; descriptive mode can
// retain compatible subsets while reporting every caveat.
func Check(before, after report.Report, mode Mode) Compatibility {
	result := Compatibility{Mode: mode, Fatal: []Mismatch{}, Warnings: []Mismatch{}, Pairs: []ExperimentPair{}}
	globalFatal := false
	add := func(field, beforeValue, afterValue, message string, fatalInDescriptive bool) {
		if beforeValue == afterValue {
			return
		}
		mismatch := Mismatch{Field: field, Before: beforeValue, After: afterValue, Message: message}
		if mode == ModeGate || fatalInDescriptive {
			result.Fatal = append(result.Fatal, mismatch)
			globalFatal = globalFatal || fatalInDescriptive
		} else {
			result.Warnings = append(result.Warnings, mismatch)
		}
	}

	beforeMajor, beforeVersionOK := schemaMajor(before.SchemaVersion)
	afterMajor, afterVersionOK := schemaMajor(after.SchemaVersion)
	if !beforeVersionOK || !afterVersionOK {
		result.Fatal = append(result.Fatal, Mismatch{Field: "schema_version", Before: before.SchemaVersion, After: after.SchemaVersion, Message: "schema versions are malformed"})
		globalFatal = true
	} else if beforeMajor != afterMajor {
		add("schema_version", before.SchemaVersion, after.SchemaVersion, "schema major versions are not comparable", true)
	} else {
		add("schema_version", before.SchemaVersion, after.SchemaVersion, "schema minor versions differ", false)
	}
	add("target.fingerprint", before.Target.Fingerprint, after.Target.Fingerprint, "target identities differ", true)
	add("configuration.method", before.Configuration.Method, after.Configuration.Method, "HTTP methods differ", true)
	add("configuration.timeout_ms", integer(before.Configuration.TimeoutMillis), integer(after.Configuration.TimeoutMillis), "request timeouts differ", false)
	add("configuration.follow_redirects", boolean(before.Configuration.FollowRedirects), boolean(after.Configuration.FollowRedirects), "redirect policies differ", false)
	add("configuration.redirect_limit", integer(int64(before.Configuration.RedirectLimit)), integer(int64(after.Configuration.RedirectLimit)), "redirect limits differ", false)
	add("configuration.body_limit_bytes", integer(before.Configuration.BodyLimitBytes), integer(after.Configuration.BodyLimitBytes), "response body limits differ", false)
	add("configuration.allow_unsafe_methods", boolean(before.Configuration.AllowUnsafeMethods), boolean(after.Configuration.AllowUnsafeMethods), "unsafe-method policies differ", false)
	add("configuration.warmups", integer(int64(before.Configuration.Warmups)), integer(int64(after.Configuration.Warmups)), "warm-up policies differ", false)
	add("configuration.rate_per_second", number(before.Configuration.Rate), number(after.Configuration.Rate), "request-start rates differ", false)
	add("configuration.concurrency_stages", integers(before.Configuration.ConcurrencyStages), integers(after.Configuration.ConcurrencyStages), "concurrency stages differ", false)
	add("rule_version", before.RuleVersion, after.RuleVersion, "diagnostic rule semantics differ", false)

	if !globalFatal {
		pairExperiments(&result, before.Experiments, after.Experiments, mode)
	}
	sort.Slice(result.Pairs, func(i, j int) bool { return result.Pairs[i].ID < result.Pairs[j].ID })
	result.DescriptiveEligible = !globalFatal && len(result.Pairs) > 0
	result.GateEligible = result.DescriptiveEligible && len(result.Fatal) == 0 && len(result.Warnings) == 0
	return result
}

func pairExperiments(result *Compatibility, before, after []report.Experiment, mode Mode) {
	beforeByID, beforeDuplicates := experimentsByID(before)
	afterByID, afterDuplicates := experimentsByID(after)
	for _, id := range append(beforeDuplicates, afterDuplicates...) {
		addExperimentMismatch(result, mode, id, "experiments.id", "unique", "duplicate", "experiment IDs must be unique")
	}

	ids := make([]string, 0, len(beforeByID)+len(afterByID))
	seen := make(map[string]struct{}, len(beforeByID)+len(afterByID))
	for id := range beforeByID {
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for id := range afterByID {
		if _, ok := seen[id]; !ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		beforeExperiment, beforeOK := beforeByID[id]
		afterExperiment, afterOK := afterByID[id]
		if !beforeOK || !afterOK {
			addExperimentMismatch(result, mode, id, "experiments.presence", boolean(beforeOK), boolean(afterOK), "experiment is absent from one report")
			continue
		}
		if beforeExperiment.Kind != afterExperiment.Kind {
			addExperimentMismatch(result, mode, id, "experiments.kind", string(beforeExperiment.Kind), string(afterExperiment.Kind), "experiment semantics differ")
			continue
		}
		if beforeExperiment.RequestedSamples != afterExperiment.RequestedSamples {
			addExperimentMismatch(result, mode, id, "experiments.requested_samples", integer(int64(beforeExperiment.RequestedSamples)), integer(int64(afterExperiment.RequestedSamples)), "experiment sample sizes differ")
		}
		if beforeExperiment.Workers != afterExperiment.Workers {
			addExperimentMismatch(result, mode, id, "experiments.workers", integer(int64(beforeExperiment.Workers)), integer(int64(afterExperiment.Workers)), "experiment worker counts differ")
		}
		if beforeExperiment.RatePerSecond != afterExperiment.RatePerSecond {
			addExperimentMismatch(result, mode, id, "experiments.rate_per_second", number(beforeExperiment.RatePerSecond), number(afterExperiment.RatePerSecond), "experiment rates differ")
		}
		result.Pairs = append(result.Pairs, ExperimentPair{ID: id, Before: beforeExperiment, After: afterExperiment})
	}
}

func addExperimentMismatch(result *Compatibility, mode Mode, id, field, before, after, message string) {
	mismatch := Mismatch{Field: field, Before: before, After: after, ExperimentID: id, Message: message}
	if mode == ModeGate {
		result.Fatal = append(result.Fatal, mismatch)
	} else {
		result.Warnings = append(result.Warnings, mismatch)
	}
}

func experimentsByID(values []report.Experiment) (map[string]report.Experiment, []string) {
	result := make(map[string]report.Experiment, len(values))
	var duplicates []string
	for _, value := range values {
		if _, exists := result[value.ID]; exists {
			duplicates = append(duplicates, value.ID)
			continue
		}
		result[value.ID] = value
	}
	return result, duplicates
}

func schemaMajor(value string) (string, bool) {
	const prefix = "sloth.report/v"
	if !strings.HasPrefix(value, prefix) {
		return "", false
	}
	parts := strings.Split(strings.TrimPrefix(value, prefix), ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", false
	}
	if _, err := strconv.Atoi(parts[0]); err != nil {
		return "", false
	}
	if _, err := strconv.Atoi(parts[1]); err != nil {
		return "", false
	}
	return parts[0], true
}

func integer(value int64) string   { return strconv.FormatInt(value, 10) }
func number(value float64) string  { return strconv.FormatFloat(value, 'g', -1, 64) }
func boolean(value bool) string    { return strconv.FormatBool(value) }
func integers(values []int) string { return fmt.Sprint(values) }
