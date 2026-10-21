# AGENTS.md

## Project context

Sloth is a lightweight HTTP diagnostic CLI written in Go. It helps developers
understand client-observable API latency through HTTP instrumentation,
controlled experiments, statistical analysis, and evidence-based diagnostics.

Sloth is not a generic HTTP client, a high-scale stress-testing framework, or a
hosted observability service. Keep the product local, bounded, and technically
honest. The core workflow is measurement -> experimentation -> interpretation
-> comparison.

Read these planning documents before changing product behavior:

- `docs/planning/context-graph.md`
- `docs/planning/decision-log.md`
- `docs/planning/roadmap.md`
- `docs/planning/issue-backlog.md`

## Git workflow

- `main` is the stable release branch. Do not commit feature work directly to it.
- `dev` is the integration and QA branch.
- Create every `SH-XX` feature branch from the latest `dev`.
- Feature pull requests target `dev`, not `main`.
- After implementing and verifying an issue, commit and push its feature branch
  unless the user explicitly asks not to.
- Stop before approving or merging any pull request unless the user explicitly
  authorizes that approval or merge.
- Promote `dev` to `main` through a dedicated pull request only after the current
  roadmap milestone gate passes. Do not mix unfinished work into that promotion.
- Do not force-push shared branches or rewrite published history unless the user
  explicitly requests it.

Milestone promotion checkpoints are:

- Foundation after SH-01 through SH-08
- Inspect after SH-09 through SH-18
- Runner and Metrics after SH-19 through SH-30
- Diagnosis after SH-31 through SH-40
- Export and Compare after SH-41 through SH-51
- Hardening after SH-52 through SH-58
- v1 Release after SH-59 through SH-62

## Commit rules

- Use a short imperative subject, ideally 50 characters and never more than 72.
- Use a single subject line only. Do not add a commit body, trailers, or
  `Co-authored-by` lines.
- Keep commits reasonably sized: one coherent, independently reviewable change.
- Do not combine unrelated refactors, formatting, documentation, and behavior.
- Do not split a single tiny logical change into noisy micro-commits.
- Start implementation commit subjects with the issue ID when one applies, for
  example `SH-16 Add inspect command`.
- Run the checks required by the issue before committing.
- Never commit secrets, generated reports, local audit notes, or benchmark claims
  without reproducible evidence.

## Work naming and scope

- Every feature or implementation unit must have a stable issue ID.
- Use short lowercase branch names in the form `SH-XX-short-description`, for
  example `SH-01-initialize-repository` or `SH-17-add-inspect-command`.
- Start issue and pull-request titles with the same ID: `SH-XX: Concise title`.
- Never reuse or renumber an assigned ID. New work takes the next available ID.
- Keep each change focused on one independently verifiable outcome.
- Record newly discovered scope as a follow-up issue instead of silently
  expanding the current change.
- Do not add production code from a planning-only issue.

## Engineering principles

1. Prefer simple, readable, idiomatic Go.
2. Keep packages focused and cohesive.
3. Separate CLI presentation from business logic.
4. Prefer the standard library when appropriate.
5. Avoid dependencies and abstractions without an identifiable benefit.
6. Use explicit configuration and centralized defaults.
7. Keep diagnostic rules deterministic and versioned.
8. Keep measurement overhead and memory use bounded and observable.
9. Favor correctness and maintainability over premature optimization.
10. Preserve the distinction between observations, hypotheses, and confirmed
    root causes.

## Code comments

Comments explain why a decision or constraint exists, not what obvious code
does. Keep comments short and technically meaningful.

Good:

```go
// Preserve planned arrival times so overload does not hide queueing delay.
```

```go
// Reuse this transport so warm measurements do not pay a new handshake.
```

Avoid:

```go
// Increment the counter.
count++
```

Use Go documentation comments for exported APIs when appropriate. Do not add
long narrative comments when a focused name or small function is clearer.

## Error handling

- Return errors rather than panicking for recoverable failures.
- Wrap errors with useful context without leaking secrets.
- Distinguish validation, timeout, transport, HTTP, interruption, report, and
  filesystem errors.
- Never silently discard a measurement failure.
- Support graceful context cancellation.

## Concurrency

- Every goroutine must have an understood lifecycle and shutdown path.
- Use contexts for cancellation and bounded worker pools for execution.
- Enforce aggregate rate limits and request budgets across all workers.
- Protect shared mutable state with the simplest suitable synchronization.
- Preserve planned and actual request start times separately.
- Test cancellation, partial failure, shutdown, and race-sensitive paths.

## Measurement and benchmarking

- Do not fabricate performance values or accuracy claims.
- Define every duration boundary and unavailable-value rule.
- Use monotonic elapsed time where available.
- Keep warm-up samples separate from measured samples.
- Record configuration and environment metadata with results.
- Do not treat correlation as proof of a server-side root cause.
- State sample-size and coordinated-omission limitations.
- Make comparison conclusions conditional on compatible workloads.
- Bound response reads, sample buffers, output size, request count, rate,
  concurrency, and timeout.

## Security and privacy

- Never log or persist authorization tokens, cookies, or request/response bodies
  by default.
- Redact sensitive URL query values before display or persistence.
- Do not add telemetry or analytics without explicit approval.
- Do not follow redirects by default.
- Require explicit acknowledgement for potentially destructive HTTP methods.
- Treat all targets as resources the user is authorized to test, while keeping
  conservative traffic defaults and hard safety ceilings.

## Testing and verification

- Add unit tests for new business logic and integration tests for HTTP behavior.
- Prefer local `httptest` servers and deterministic synthetic measurements.
- Cover positive, negative, boundary, cancellation, and failure paths.
- Avoid assertions that depend on tight wall-clock tolerances.
- Run `go test ./...`, `go test -race ./...`, `go vet ./...`, and formatting or
  lint checks appropriate to the changed scope.
- Verify machine-readable output contains no ANSI control sequences.

## Documentation

Update relevant documentation when commands, flags, defaults, timing semantics,
diagnostic rules, safety limits, or the report schema change. Architecture and
methodology documentation are part of the feature, not deferred cleanup.

## Scope control

Do not introduce a frontend, cloud infrastructure, database, authentication
service, unrestricted load generation, LLM-based diagnosis, or distributed
tracing backend without an approved scope change. A local sample HTTP server is
acceptable only as an example or deterministic test fixture; Sloth itself does
not run a server.
