package report

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"runtime"
	"sort"
	"time"

	"github.com/ansonnchan/sloth/internal/buildinfo"
	"github.com/ansonnchan/sloth/internal/config"
	"github.com/ansonnchan/sloth/internal/diagnose"
	"github.com/ansonnchan/sloth/internal/diagnosis"
	"github.com/ansonnchan/sloth/internal/experiment"
	"github.com/ansonnchan/sloth/internal/httpmeasure"
	"github.com/ansonnchan/sloth/internal/metrics"
	"github.com/ansonnchan/sloth/internal/runner"
)

type BuildInput struct {
	Result        diagnose.Result
	Target        httpmeasure.TargetIdentity
	Configuration config.Snapshot
	Tool          buildinfo.Info
	RunID         string
	CreatedAt     time.Time
	Environment   Environment
}

func Build(input BuildInput) (Report, error) {
	if input.Configuration.IncludeSamples && (input.Configuration.SampleLimit <= 0 || input.Configuration.SampleLimit > MaxExportedSamples) {
		return Report{}, fmt.Errorf("sample limit must be between 1 and %d", MaxExportedSamples)
	}
	runID := input.RunID
	if runID == "" {
		var err error
		runID, err = newRunID()
		if err != nil {
			return Report{}, err
		}
	}
	createdAt := input.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	createdAt = createdAt.UTC()
	environment := input.Environment
	if environment.OS == "" {
		environment = CurrentEnvironment()
	}

	report := Report{
		SchemaVersion: SchemaVersion,
		ToolVersion:   input.Tool.Version,
		RuleVersion:   input.Result.Diagnosis.RuleVersion,
		Tool:          ToolFromBuildInfo(input.Tool),
		Run:           Run{ID: runID, CreatedAt: createdAt},
		Target:        input.Target,
		Configuration: input.Configuration,
		Environment:   environment,
		Diagnostics:   append([]diagnosis.Finding(nil), input.Result.Diagnosis.Findings...),
		Limitations:   []string{},
	}
	if input.Configuration.IncludeSamples {
		report.SampleExport = &SampleExport{Limit: input.Configuration.SampleLimit}
		report.Samples = []Sample{}
	}

	appendExperiment := func(result runner.ExperimentReport) {
		if result.Result.Plan.ID() == "" && result.Aggregate.Outcomes.Attempted == 0 {
			return
		}
		experimentReport := newExperiment(result)
		report.Experiments = append(report.Experiments, experimentReport)
		accumulateOverall(&report.Overall, result)
		for _, limitation := range experimentReport.Limitations {
			report.Limitations = appendUnique(report.Limitations, experimentReport.ID+": "+limitation)
		}
		updateRunBounds(&report.Run, result.Result.Execution)
		appendSamples(&report, result)
	}
	appendExperiment(input.Result.Suite.Baseline)
	appendExperiment(input.Result.Suite.Connection.Fresh)
	appendExperiment(input.Result.Suite.Connection.Reused)
	for _, result := range input.Result.Suite.Concurrency {
		appendExperiment(result)
	}
	for _, limitation := range input.Result.Suite.Connection.Limitations {
		report.Limitations = appendUnique(report.Limitations, "connection comparison: "+limitation)
	}
	for _, finding := range report.Diagnostics {
		if finding.Code == diagnosis.CodeInsufficientEvidence {
			for _, limitation := range finding.Evidence.Notes {
				report.Limitations = appendUnique(report.Limitations, limitation)
			}
		}
	}
	if report.Experiments == nil {
		report.Experiments = []Experiment{}
	}
	if report.Diagnostics == nil {
		report.Diagnostics = []diagnosis.Finding{}
	}
	if report.Overall.WarmupOutcomes == nil {
		report.Overall.WarmupOutcomes = map[string]int{}
	}
	if report.SampleExport != nil {
		report.SampleExport.Included = len(report.Samples)
		report.SampleExport.Truncated = report.SampleExport.Included < report.SampleExport.Available
	}
	return report, nil
}

func CurrentEnvironment() Environment {
	return Environment{OS: runtime.GOOS, Arch: runtime.GOARCH, GoVersion: runtime.Version(), CPUs: runtime.NumCPU()}
}

func newRunID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", fmt.Errorf("generate report run ID: %w", err)
	}
	return hex.EncodeToString(value), nil
}

func newExperiment(source runner.ExperimentReport) Experiment {
	plan := source.Result.Plan
	limitations := append([]string(nil), source.Aggregate.Limitations...)
	if limitations == nil {
		limitations = []string{}
	}
	return Experiment{
		ID: plan.ID(), Kind: plan.Kind(), RequestedSamples: plan.Requests(), WarmupRequests: plan.Warmups(),
		Workers: plan.Workers(), RatePerSecond: plan.Rate(), WarmupOutcomes: outcomeSummary(source.Result.WarmupSummary),
		Execution: execution(source.Result.Execution), Aggregate: aggregate(source.Aggregate), Limitations: limitations,
	}
}

func execution(value experiment.Execution) Execution {
	return Execution{
		StartedAt: value.StartedAt.UTC(), FinishedAt: value.FinishedAt.UTC(), OfferedStarts: value.OfferedStarts,
		AchievedStarts: value.AchievedStarts, MissedArrivals: value.MissedArrivals,
		Interrupted: value.Interrupted, StopReason: value.StopReason,
	}
}

func aggregate(value metrics.Aggregate) Aggregate {
	phases := make(map[string]Statistics, len(value.Phases))
	keys := make([]string, 0, len(value.Phases))
	for key := range value.Phases {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		phases[key] = statistics(value.Phases[key])
	}
	return Aggregate{
		Outcomes: value.Outcomes, StatusClasses: cloneMap(value.StatusClasses), ResponseBytes: value.ResponseBytes,
		Latency: statistics(value.Latency), SchedulingDelay: statistics(value.SchedulingDelay), Phases: phases,
		Rates: Rates{
			DurationMillis: milliseconds(value.Rates.Duration), OfferedStarts: value.Rates.OfferedStarts,
			AchievedStarts: value.Rates.AchievedStarts, OfferedPerSecond: value.Rates.OfferedPerSec,
			AchievedPerSecond: value.Rates.AchievedPerSec, CompletedPerSecond: value.Rates.Throughput,
		},
		Concurrency: value.Concurrency, Protocols: cloneMap(value.Protocols), Connections: value.Connections,
		Interrupted: value.Interrupted,
	}
}

func statistics(value metrics.LatencyStatistics) Statistics {
	result := Statistics{Count: value.Count}
	if !value.Available {
		return result
	}
	result.MinMillis = durationPointer(value.Min)
	result.MaxMillis = durationPointer(value.Max)
	result.MeanMillis = durationPointer(value.Mean)
	result.P50Millis = durationPointer(value.Median)
	result.P90Millis = durationPointer(value.P90)
	result.P95Millis = durationPointer(value.P95)
	result.P99Millis = durationPointer(value.P99)
	result.PopulationStdDevMillis = durationPointer(value.PopulationStdDev)
	return result
}

func durationPointer(value time.Duration) *float64 {
	millis := milliseconds(value)
	return &millis
}

func milliseconds(value time.Duration) float64 { return float64(value) / float64(time.Millisecond) }

func outcomeSummary(value experiment.OutcomeSummary) map[string]int {
	result := make(map[string]int, len(value))
	for key, count := range value {
		result[string(key)] = count
	}
	return result
}

func cloneMap(value map[string]int) map[string]int {
	result := make(map[string]int, len(value))
	for key, count := range value {
		result[key] = count
	}
	return result
}

func accumulateOverall(overall *Overall, source runner.ExperimentReport) {
	counts := source.Aggregate.Outcomes
	overall.MeasuredOutcomes.Attempted += counts.Attempted
	overall.MeasuredOutcomes.Completed += counts.Completed
	overall.MeasuredOutcomes.Successful += counts.Successful
	overall.MeasuredOutcomes.HTTPError += counts.HTTPError
	overall.MeasuredOutcomes.TransportError += counts.TransportError
	overall.MeasuredOutcomes.Timeout += counts.Timeout
	overall.MeasuredOutcomes.Cancelled += counts.Cancelled
	overall.MeasuredOutcomes.BodyLimit += counts.BodyLimit
	overall.ResponseBytes += source.Aggregate.ResponseBytes
	if overall.WarmupOutcomes == nil {
		overall.WarmupOutcomes = make(map[string]int)
	}
	for category, count := range source.Result.WarmupSummary {
		overall.WarmupOutcomes[string(category)] += count
		overall.WarmupAttempts += count
	}
	if source.Aggregate.Interrupted {
		overall.InterruptedExperiments++
	}
}

func updateRunBounds(run *Run, value experiment.Execution) {
	if !value.StartedAt.IsZero() && (run.StartedAt.IsZero() || value.StartedAt.Before(run.StartedAt)) {
		run.StartedAt = value.StartedAt.UTC()
	}
	if !value.FinishedAt.IsZero() && (run.FinishedAt.IsZero() || value.FinishedAt.After(run.FinishedAt)) {
		run.FinishedAt = value.FinishedAt.UTC()
	}
	run.Interrupted = run.Interrupted || value.Interrupted
}

func appendUnique(values []string, candidate string) []string {
	for _, value := range values {
		if value == candidate {
			return values
		}
	}
	return append(values, candidate)
}

func appendSamples(destination *Report, source runner.ExperimentReport) {
	if destination.SampleExport == nil {
		return
	}
	destination.SampleExport.Available += len(source.Result.Measured)
	for _, value := range source.Result.Measured {
		if len(destination.Samples) >= destination.SampleExport.Limit {
			return
		}
		destination.Samples = append(destination.Samples, sample(value))
	}
}

func sample(value experiment.Sample) Sample {
	var statusCode *int
	if value.StatusCode != 0 {
		code := value.StatusCode
		statusCode = &code
	}
	return Sample{
		ExperimentID: value.ExperimentID, Sequence: value.Sequence,
		PlannedStart: value.PlannedStart.UTC(), ActualStart: value.ActualStart.UTC(), FinishedAt: value.FinishedAt.UTC(),
		SchedulingDelayMillis: milliseconds(value.SchedulingDelay()),
		Timings: SampleTimings{
			Total: sampleDuration(value.Timings.Total), DNS: sampleDuration(value.Timings.DNS),
			Connect: sampleDuration(value.Timings.Connect), TLS: sampleDuration(value.Timings.TLS),
			RequestWrite: sampleDuration(value.Timings.RequestWrite), FirstByteWait: sampleDuration(value.Timings.FirstByteWait),
			BodyTransfer: sampleDuration(value.Timings.BodyTransfer),
		},
		Outcome: SampleOutcome{
			Category: value.Outcome.Category, FailurePhase: value.Outcome.FailurePhase,
			ResponseReceived: value.Outcome.ResponseReceived, Complete: value.Outcome.Complete,
		},
		Connection: value.Connection, Protocol: value.Protocol, StatusCode: statusCode, ResponseBytes: value.ResponseBytes,
	}
}

func sampleDuration(value httpmeasure.PhaseDuration) SampleDuration {
	duration, available := value.Value()
	if !available {
		return SampleDuration{UnavailableReason: value.Reason()}
	}
	return SampleDuration{DurationMillis: durationPointer(duration)}
}
