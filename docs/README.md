# Sloth technical documentation

Start with the [command reference](commands.md) to run Sloth or the [methodology](methodology.md) to interpret a result. The documents below are part of the product contract: changes to public behavior must update the matching document and tests.

## Use Sloth

- [Commands and flags](commands.md): syntax, defaults, safety behavior, and examples for every command.
- [Configuration and safety ceilings](configuration.md): validation, traffic budgets, and acknowledgement boundaries.
- [Local sample API](../examples/sample-api/README.md): reproducible targets for inspect, diagnose, export, and compare.
- [CLI streams and exit codes](cli-contract.md): stdout/stderr separation, terminal behavior, and automation statuses.
- [Network failures](network-failures.md): observable failure categories and partial evidence.

## Interpret evidence

- [Methodology and limitations](methodology.md): experimental design, statistical interpretation, and coordinated omission.
- [Timing semantics](timing.md) and [metric definitions](metrics.md): exact measurement boundaries and formulas.
- [Diagnostic rules](diagnostics.md): thresholds, confidence, false positives, and recommendations.
- [Experiment runner](runner.md): planned arrivals, shared rate limiting, worker bounds, and warm-ups.

## Persist and automate

- [Report model](report-model.md), [files](report-files.md), and [schema evolution](schema/README.md).
- [Report compatibility](report-compatibility.md), [deltas](report-deltas.md), and [reading guidance](report-reading.md).
- [Inspect JSON](inspect-json.md) and [diagnose JSON](diagnose-json.md) for transient command output.
- [Benchmarking](benchmarking.md) for reproducible client-overhead evidence.

## Build and maintain

- [Architecture](architecture.md): data flow, ownership, cancellation, and package boundaries.
- [Security threat model](security-threat-model.md), [fuzzing](fuzzing.md), and [build metadata](builds.md).
- [Supported platforms](platforms.md): native smoke coverage and filesystem/signal limitations.
- [Planning](planning/README.md): roadmap, issue dependencies, and historical decisions.

## Behavior traceability

| Public behavior | Source issues | Primary implementation | Primary verification |
|---|---|---|---|
| Configuration, safety, streams, CI | SH-01–SH-08 | `internal/config`, `internal/safety`, `internal/cli` | package tests and `.github/workflows/ci.yml` |
| Request model, tracing, timing, inspect | SH-09–SH-18 | `internal/httpmeasure`, `internal/inspect` | `internal/httpmeasure/*_test.go`, `internal/cli/inspect*_test.go` |
| Scheduling, experiments, aggregation | SH-19–SH-30 | `internal/runner`, `internal/experiment`, `internal/metrics` | runner lifecycle/race tests and metrics boundary tests |
| Diagnostic rules | SH-31–SH-40 | `internal/diagnosis`, `internal/diagnose` | rule threshold and deterministic-order tests |
| Reports, export, comparison, CI gate | SH-41–SH-51 | `internal/report`, `internal/compare` | schema fixtures and CLI export/compare tests |
| Accessibility, privacy, fuzzing, failures | SH-52–SH-55 | CLI/render/config/report/network boundaries | accessibility, privacy, fuzz, and network-failure tests |
| Benchmarks and sample API | SH-56–SH-57 | benchmark suites and `examples/sample-api` | ceiling tests and `sample_api_smoke_test.go` |
| Complete technical documentation | SH-58 | `docs/` | `internal/cli/docs_test.go` plus link/schema tests |
