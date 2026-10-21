# Delivery roadmap

## Planning assumptions

- Issue IDs are permanent and dependency ordered, not a promise that every issue
  is equal in size.
- A milestone may begin only when its required upstream contracts are stable.
- Documentation and tests named in an issue are part of its acceptance criteria.
- The first release favors exact bounded data over unbounded scale.
- Post-v1 ideas remain outside the release gate unless explicitly promoted.

## Dependency spine

```text
Foundation
  -> request measurement
      -> bounded runner + metrics
          -> deterministic diagnosis
              -> export + comparison
                  -> hardening
                      -> release

Privacy, cancellation, safety budgets, documentation, and tests cut across all
milestones rather than appearing only at the end.
```

## Milestone 1 — Foundation (`SH-01` through `SH-08`)

Outcome: a buildable, testable CLI skeleton with central configuration, public
exit/output contracts, traffic safeguards, and required CI checks.

Release gate:

- `sloth --help` and `sloth --version` execute successfully.
- Config validation happens before network activity.
- CI runs format, vet, tests, race detection, and builds.
- No production HTTP server or empty package scaffolding is introduced.

## Milestone 2 — Single-request measurement (`SH-09` through `SH-18`)

Outcome: `sloth inspect` accurately captures and explains one bounded request
without leaking sensitive data.

Release gate:

- HTTP trace events are captured without assuming every event exists once.
- Timing derivation handles reused connections, HTTP/2, proxies, failures, and
  unavailable phases.
- Redirect and response-body behavior are explicit and bounded.
- Human and JSON inspect outputs pass local integration tests.

## Milestone 3 — Experiment runner and metrics (`SH-19` through `SH-30`)

Outcome: repeatable experiments run under a global rate limit and bounded worker
pool while preserving scheduling delay and actual concurrency evidence.

Release gate:

- Worker, rate, budget, warm-up, and cancellation semantics are tested.
- Exact percentile and aggregate definitions are documented.
- Baseline, connection, and concurrency experiments emit the same structured
  result model.
- Concurrent code passes the race detector without leaks.

## Milestone 4 — Automated diagnosis (`SH-31` through `SH-40`)

Outcome: `sloth diagnose` runs the standard suite and emits deterministic,
evidence-backed findings for all six required rules.

Release gate:

- Thresholds and rule version are centralized.
- Each rule has positive, negative, boundary, contradictory, and sparse-data
  coverage.
- Findings distinguish observations from possible causes.
- Progress never contaminates structured output.

## Milestone 5 — Export and comparison (`SH-41` through `SH-51`)

Outcome: reports are safely persisted and compatible results can be compared or
used as a CI regression gate.

Release gate:

- The report model and JSON Schema agree and validate fixtures.
- Writes are atomic and never overwrite without `--force`.
- Compatibility is checked before deltas or gate decisions.
- Zero baselines, sparse evidence, and interrupted reports are handled honestly.

## Milestone 6 — Hardening and documentation (`SH-52` through `SH-58`)

Outcome: the tool behaves predictably across failure modes, terminals, and
platforms, and its methodology is understandable without reading source code.

Release gate:

- Security, privacy, fuzz, integration, and cancellation suites pass.
- Self-overhead and scheduler behavior have reproducible benchmarks.
- Accessibility and noninteractive output requirements are verified.
- All command, architecture, methodology, diagnostic, benchmarking, and example
  documentation is complete.

## Milestone 7 — Public release (`SH-59` through `SH-62`)

Outcome: signed-off v1 artifacts are reproducibly built for supported targets
and a developer can install and use the four core workflows from the README.

Release gate:

- GoReleaser archives and checksums are verified.
- The platform matrix builds and smoke-tests.
- The terminal demo reflects real output.
- The v1 readiness checklist has evidence for every product success criterion.

## Post-v1 candidates (`SH-63` through `SH-68`)

These are improvements derived during PRD analysis, not v1 commitments:

- Offline re-analysis of saved evidence
- Repeated-run uncertainty analysis
- Phase-level regression gates
- Portable CI summary output
- Homebrew publishing
- Richer proxy and address-family observations

## Suggested GitHub organization

Milestones:

- `M1 Foundation`
- `M2 Inspect`
- `M3 Runner & Metrics`
- `M4 Diagnosis`
- `M5 Export & Compare`
- `M6 Hardening`
- `M7 v1 Release`
- `Post-v1`

Labels:

- Type: `type:feature`, `type:test`, `type:docs`, `type:chore`, `type:research`
- Area: `area:cli`, `area:http`, `area:runner`, `area:metrics`,
  `area:diagnosis`, `area:report`, `area:security`, `area:release`
- Priority: `priority:critical`, `priority:high`, `priority:normal`
- State aids: `blocked`, `needs-decision`, `post-v1`

GitHub dependencies should mirror the `Depends on` fields in the issue backlog.
