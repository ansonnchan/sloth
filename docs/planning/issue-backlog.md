# Publish-ready issue backlog

Each section is one GitHub issue. Titles, suggested branch names, dependencies,
and milestone/label metadata are part of the planning contract. When these are
published, preserve the issue ID even if GitHub assigns a different number.

## SH-01: Initialize repository and bootable CLI

**Branch:** `SH-01-initialize-repository`  
**Milestone:** M1 Foundation  
**Labels:** `type:chore`, `area:cli`, `priority:critical`  
**Depends on:** None

**What:** Create the minimal idiomatic Go repository and a Cobra-based `sloth`
entry point.

**Why:** Every later package needs a real module, executable boundary, and test
command. Sloth is a CLI, so the foundation is a runnable binary rather than a
production server.

**Suggested approach:** Initialize `github.com/ansonnchan/sloth`, add only the
root command and files needed to build, and keep domain packages out until their
own issues. Use build-time variables for a development version placeholder.

**Acceptance criteria:**

- [ ] `go run ./cmd/sloth --help` exits successfully and names the product.
- [ ] `go run ./cmd/sloth --version` prints a nonempty development version.
- [ ] `go test ./...` and `go vet ./...` pass.
- [ ] The repository contains `LICENSE`, `.gitignore`, a concise README skeleton,
  and no empty source-file scaffolding.
- [ ] No persistent HTTP server or external infrastructure is introduced.

## SH-02: Centralize defaults and effective configuration

**Branch:** `SH-02-centralize-configuration`  
**Milestone:** M1 Foundation  
**Labels:** `type:feature`, `area:cli`, `priority:critical`  
**Depends on:** SH-01

**What:** Define typed shared configuration and one source for command defaults.

**Why:** Duplicated defaults make runs difficult to reproduce and commands drift
over time.

**Suggested approach:** Keep command-agnostic values in `internal/config`; map
Cobra flags into an effective immutable config before orchestration.

**Acceptance criteria:**

- [ ] Defaults cover method, timeout, request count, warm-ups, rate, concurrency
  stages, redirects, color, body cap, and sample inclusion.
- [ ] Commands do not duplicate default constants.
- [ ] Effective configuration can be serialized without secrets.
- [ ] Unit tests prove flag overrides do not mutate global defaults.

## SH-03: Validate targets, methods, and flag combinations

**Branch:** `SH-03-validate-command-inputs`  
**Milestone:** M1 Foundation  
**Labels:** `type:feature`, `area:cli`, `priority:critical`  
**Depends on:** SH-02

**What:** Add reusable validation that runs before filesystem or network work.

**Why:** Invalid URLs, counts, durations, methods, and conflicting flags should
fail predictably and explain how to fix the input.

**Suggested approach:** Return typed validation errors from pure functions;
accept only HTTP/HTTPS URLs and the documented method set in v1.

**Acceptance criteria:**

- [ ] Invalid schemes, missing hosts, malformed headers, methods, counts, rates,
  concurrency, and timeouts are rejected before any request.
- [ ] Cross-flag conflicts have actionable messages with a valid example.
- [ ] URL userinfo and secret-like query values are absent from error text.
- [ ] Table-driven tests cover valid, invalid, and boundary inputs.

## SH-04: Enforce traffic budgets and unsafe-method acknowledgement

**Branch:** `SH-04-enforce-safety-budgets`  
**Milestone:** M1 Foundation  
**Labels:** `type:feature`, `area:security`, `priority:critical`, `needs-decision`  
**Depends on:** SH-02, SH-03

**What:** Ratify safety ceilings and require explicit acknowledgement for
side-effecting or unusually large runs.

**Why:** A diagnostic tool must not silently become an unrestricted load
generator or repeat destructive methods.

**Suggested approach:** Calculate the complete suite budget, including warm-ups,
then enforce documented normal and absolute ceilings. Use noninteractive flags,
not prompts, so CI behavior is stable.

**Acceptance criteria:**

- [ ] Normal and absolute ceilings for requests, rate, concurrency, timeout,
  response bytes, samples, and redirect hops are documented and tested.
- [ ] POST, PUT, PATCH, DELETE, and other non-safe methods require
  `--allow-unsafe-methods` before any request.
- [ ] Exceeding a normal ceiling requires an explicit high-load acknowledgement;
  absolute ceilings cannot be bypassed.
- [ ] Budget output states total planned requests, including warm-ups and every
  experiment arm.

## SH-05: Define output streams, quiet behavior, and exit codes

**Branch:** `SH-05-define-cli-contract`  
**Milestone:** M1 Foundation  
**Labels:** `type:feature`, `area:cli`, `priority:high`, `needs-decision`  
**Depends on:** SH-01

**What:** Establish the public stdout, stderr, and process-exit contract.

**Why:** Shell scripts and CI need to distinguish data, progress, usage errors,
runtime failures, interruptions, and regression findings.

**Suggested approach:** Centralize command execution and error-to-exit mapping;
ratify D-003 and D-014 from the decision log.

**Acceptance criteria:**

- [ ] Structured output uses stdout only; progress and warnings use stderr.
- [ ] Exit codes are documented for success, runtime failure, validation,
  regression, and interruption.
- [ ] Cobra does not print duplicate errors or usage for runtime failures.
- [ ] Subprocess tests verify streams and codes.

## SH-06: Add reproducible version and build metadata

**Branch:** `SH-06-add-build-metadata`  
**Milestone:** M1 Foundation  
**Labels:** `type:chore`, `area:cli`, `area:release`  
**Depends on:** SH-01

**What:** Define tool version, commit, build time, and dirty-state injection.

**Why:** Exported reports and support requests need to identify the exact Sloth
build that produced them.

**Suggested approach:** Provide safe development fallbacks and linker-set values
for releases; keep report-facing metadata independent of Cobra.

**Acceptance criteria:**

- [ ] `sloth --version` works for local and release builds.
- [ ] Version data is available as a typed value to report generation.
- [ ] Builds without linker flags remain valid and say `dev` rather than empty.
- [ ] Tests do not depend on the current Git checkout.

## SH-07: Establish CI quality gates

**Branch:** `SH-07-add-ci-quality-gates`  
**Milestone:** M1 Foundation  
**Labels:** `type:chore`, `area:release`, `priority:critical`  
**Depends on:** SH-01

**What:** Add GitHub Actions for formatting, vetting, tests, race detection, and
build validation.

**Why:** Concurrency and serialization defects should block merging from the
first implementation milestone.

**Suggested approach:** Use pinned action versions, Go module caching, separate
race and cross-build jobs, and minimal token permissions.

**Acceptance criteria:**

- [ ] Pull requests and pushes to `main` run format, vet, unit/integration tests,
  race detection, and builds.
- [ ] Workflow permissions follow least privilege.
- [ ] Jobs have timeouts and cancellation for superseded runs.
- [ ] A failing required check produces a nonzero job result.

## SH-08: Add contributor workflow and repository templates

**Branch:** `SH-08-add-contributor-workflow`  
**Milestone:** M1 Foundation  
**Labels:** `type:docs`, `type:chore`  
**Depends on:** SH-01, SH-07

**What:** Add contribution guidance, issue forms, pull-request checks, and common
developer commands.

**Why:** The issue ID, branch naming, verification, and documentation rules must
be discoverable outside `AGENTS.md`.

**Suggested approach:** Keep templates short and link to canonical planning and
methodology documents rather than duplicating them.

**Acceptance criteria:**

- [ ] CONTRIBUTING documents `SH-XX-short-description` naming and required checks.
- [ ] Feature and bug issue forms request scope and acceptance evidence.
- [ ] The PR template requests linked issue, tests, docs, and safety/privacy impact.
- [ ] Local development commands match CI commands.

## SH-09: Model request specifications and redacted target identity

**Branch:** `SH-09-model-http-requests`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `area:security`, `priority:critical`  
**Depends on:** SH-02, SH-03

**What:** Define the runtime request specification and a separate safe target
identity for rendering and persistence.

**Why:** Raw URLs and headers may contain secrets, while comparisons still need
a stable endpoint identity.

**Suggested approach:** Canonicalize scheme/host/port/path, retain query keys but
redact values, and compute fingerprints only from the redacted representation.

**Acceptance criteria:**

- [ ] Runtime-only request data is structurally separate from persisted metadata.
- [ ] Authorization, Proxy-Authorization, Cookie, Set-Cookie, URL userinfo, and
  configured secret-like query values are redacted.
- [ ] Equivalent canonical targets produce the same safe fingerprint.
- [ ] Tests prove secrets do not appear in formatted values or errors.

## SH-10: Define request outcomes and error taxonomy

**Branch:** `SH-10-classify-request-outcomes`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `priority:critical`  
**Depends on:** SH-09

**What:** Introduce typed success, HTTP error, timeout, transport, cancellation,
and body-limit outcomes.

**Why:** Metrics and diagnostics must not confuse a completed HTTP 500 with a
network failure or user cancellation.

**Suggested approach:** Preserve wrapped Go errors internally while exposing a
stable public category and redacted message.

**Acceptance criteria:**

- [ ] HTTP 4xx/5xx responses retain timings and status metadata.
- [ ] Context deadlines, user cancellation, DNS/TLS/connect failures, and body
  limit outcomes are distinguishable.
- [ ] Partial responses preserve valid evidence and identify incompleteness.
- [ ] Table-driven tests cover wrapping and secret-safe messages.

## SH-11: Capture raw HTTP trace observations

**Branch:** `SH-11-capture-http-traces`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `priority:critical`  
**Depends on:** SH-09, SH-10

**What:** Capture `httptrace` callbacks and request lifecycle markers into a
request-scoped trace record.

**Why:** Accurate derived timings require raw evidence that does not assume one
DNS lookup, one connection attempt, or a fresh connection.

**Suggested approach:** Record monotonic event times and repeated attempts under
a request-local collector; avoid doing expensive work inside callbacks.

**Acceptance criteria:**

- [ ] DNS, connect, TLS, GotConn, request-write, and first-byte events are captured.
- [ ] Multiple connect attempts and missing/out-of-order hooks do not panic.
- [ ] Reuse, idle state, protocol observations, and callback errors are retained.
- [ ] Concurrent requests cannot mutate one another's trace records.

## SH-12: Derive timing phases with availability semantics

**Branch:** `SH-12-derive-timing-phases`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `area:metrics`, `priority:critical`  
**Depends on:** SH-11

**What:** Convert raw events into documented phase durations and availability
reasons.

**Why:** Zero-filling missing hooks or summing overlapping intervals would make
the central product claim misleading.

**Suggested approach:** Use explicit optional-duration types, validate monotonic
ordering, and preserve enough raw context to explain unavailable values.

**Acceptance criteria:**

- [ ] Total, DNS, connect, TLS, request-write/first-byte wait, and body-transfer
  boundaries match `context-graph.md`.
- [ ] Reused and cleartext connections show inapplicable phases as unavailable.
- [ ] Negative, overlapping, repeated, and incomplete event sequences are handled
  conservatively and tested.
- [ ] Documentation states phase values may not add exactly to total duration.

## SH-13: Build explicit HTTP transport profiles

**Branch:** `SH-13-configure-http-transports`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `priority:critical`  
**Depends on:** SH-09

**What:** Create isolated, reusable transport factories for default, fresh, and
warm connection behavior.

**Why:** Connection experiments are invalid if transport state leaks between
arms or HTTP/2 behavior is mistaken for independent TCP connections.

**Suggested approach:** Clone safe transport defaults, explicitly configure pool
limits, proxy behavior, TLS, keep-alives, and cleanup ownership.

**Acceptance criteria:**

- [ ] Default behavior supports HTTP/1.1, HTTP/2, HTTPS, proxies, IPv4, and IPv6.
- [ ] Fresh and reused profiles cannot share connection pools.
- [ ] Idle connections are closed at the end of their owner lifecycle.
- [ ] Tests verify reuse observations without relying on exact port numbers.

## SH-14: Bound and drain response bodies safely

**Branch:** `SH-14-bound-response-bodies`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `area:security`, `priority:critical`  
**Depends on:** SH-10, SH-13

**What:** Stream response bodies without retention, enforce a byte cap, and close
them on every path.

**Why:** Unbounded reads can exhaust memory, while incomplete draining changes
connection reuse and therefore the measurement.

**Suggested approach:** Ratify D-016, count transferred bytes independently of
Content-Length, and make truncation/reuse consequences explicit.

**Acceptance criteria:**

- [ ] Bodies are never stored by default and memory use is bounded.
- [ ] Complete bodies are drained and closed so reuse remains meaningful.
- [ ] Cap exceedance is a typed outcome with transferred-byte evidence.
- [ ] Empty, chunked, compressed, streaming, partial, and oversized bodies are
  covered by local tests.

## SH-15: Implement bounded redirect policy

**Branch:** `SH-15-control-http-redirects`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:http`, `area:security`  
**Depends on:** SH-09, SH-13

**What:** Disable redirects by default and add an explicit bounded opt-in.

**Why:** Redirects change destinations, privacy boundaries, methods, and timing
interpretation.

**Suggested approach:** Return the initial 3xx as a normal response by default;
when enabled, record each hop and apply header/method safety rules.

**Acceptance criteria:**

- [ ] Default inspection returns the first redirect response without following it.
- [ ] Opt-in redirect following enforces a configured absolute hop cap.
- [ ] Cross-origin hops do not forward sensitive headers.
- [ ] Reports expose redirect hops and final redacted target identity.

## SH-16: Orchestrate the inspect command

**Branch:** `SH-16-add-inspect-command`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:cli`, `area:http`, `priority:critical`  
**Depends on:** SH-03, SH-10, SH-12, SH-14, SH-15

**What:** Wire one validated request through measurement into a structured inspect
result.

**Why:** This is the first end-to-end user workflow and the basis for later
experiments.

**Suggested approach:** Keep Cobra limited to parsing and dispatch; place
orchestration behind a context-aware service that accepts injectable clients.

**Acceptance criteria:**

- [ ] GET, HEAD, POST, PUT, PATCH, and DELETE work subject to safety policy.
- [ ] Method, timeout, repeatable headers, body file, redirects, and cancellation
  are honored.
- [ ] Timeout and Ctrl+C terminate promptly without leaking goroutines.
- [ ] The command never stores request or response bodies in its result.

## SH-17: Render readable inspect output

**Branch:** `SH-17-render-inspect-output`  
**Milestone:** M2 Inspect  
**Labels:** `type:feature`, `area:cli`  
**Depends on:** SH-12, SH-16

**What:** Produce the calm, compact terminal timing view and one bounded
interpretation.

**Why:** Raw trace events do not answer the user's question, but the UI must not
overstate what the client can know.

**Suggested approach:** Render from a view model; use width-aware layouts and a
small deterministic interpretation selector.

**Acceptance criteria:**

- [ ] Output includes method, redacted endpoint, status, protocol, total, phases,
  reuse state, response bytes, and one interpretation.
- [ ] Missing phases render as `N/A`, never `0 ms`.
- [ ] Narrow-terminal and ASCII/no-color snapshots remain readable.
- [ ] Wording never labels first-byte wait as pure server execution time.

## SH-18: Add inspect JSON output and integration coverage

**Branch:** `SH-18-test-inspect-workflow`  
**Milestone:** M2 Inspect  
**Labels:** `type:test`, `area:http`, `area:report`, `priority:high`  
**Depends on:** SH-16, SH-17

**What:** Define machine-readable inspect output and validate the complete
workflow against local servers.

**Why:** Instrumentation correctness must be demonstrated across response and
failure modes before repeated experiments amplify it.

**Suggested approach:** Use deterministic `httptest` handlers for fast, delayed
headers, slow body, errors, redirects, disconnects, cancellation, and HTTP/2.

**Acceptance criteria:**

- [ ] `inspect --json` emits one valid ANSI-free document on stdout.
- [ ] JSON distinguishes null/unavailable phases from zero durations.
- [ ] Integration cases cover 2xx, 4xx, 5xx, timeout, redirect, partial body,
  reuse, and HTTP/2 where supported.
- [ ] Tests assert relationships and classifications rather than fragile exact
  wall-clock durations.

## SH-19: Define experiment plans, samples, and results

**Branch:** `SH-19-model-experiments`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `priority:critical`  
**Depends on:** SH-02, SH-10, SH-12

**What:** Introduce shared types for an experiment plan, warm-up and measured
samples, execution metadata, and aggregate result.

**Why:** Baseline, connection, and concurrency experiments need comparable data
without depending on CLI or report schemas.

**Suggested approach:** Use stable experiment kinds and immutable effective
config snapshots; keep samples bounded and preserve partial/interrupted state.

**Acceptance criteria:**

- [ ] Warm-up and measured samples cannot be accidentally aggregated together.
- [ ] Each sample has experiment, sequence, planned start, actual start, timing,
  outcome, connection, protocol, and response metadata.
- [ ] Partial results record why execution stopped.
- [ ] Domain types do not import Cobra or terminal packages.

## SH-20: Schedule planned and actual request starts

**Branch:** `SH-20-add-arrival-scheduler`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `priority:critical`  
**Depends on:** SH-19

**What:** Implement an arrival scheduler that preserves intended start times and
records dispatch delay.

**Why:** Under overload, silently shifting arrivals hides coordinated omission
and makes endpoint latency look better than the user experience.

**Suggested approach:** Generate a bounded schedule from a monotonic origin and
hand work to the pool only through context-aware channels.

**Acceptance criteria:**

- [ ] Every planned request has an intended and actual start time.
- [ ] Scheduling delay is reported separately from HTTP duration.
- [ ] Missed arrivals are counted rather than discarded.
- [ ] Tests use tolerances and fake/injectable clocks where practical.

## SH-21: Enforce one aggregate request-start rate

**Branch:** `SH-21-add-global-rate-limit`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `priority:critical`  
**Depends on:** SH-20, SH-04

**What:** Apply the configured request-start rate across workers, warm-ups, and
all experiment stages.

**Why:** Per-worker limiters would multiply traffic and violate the PRD's safety
and comparability guarantees.

**Suggested approach:** Own one limiter at suite scope and make token acquisition
cancellable; record offered and achieved starts.

**Acceptance criteria:**

- [ ] Concurrency does not multiply the configured aggregate start rate.
- [ ] Warm-ups consume rate capacity.
- [ ] Cancellation while waiting for a start returns promptly.
- [ ] Deterministic tests verify burst and sustained behavior within documented
  tolerance.

## SH-22: Implement bounded workers and graceful shutdown

**Branch:** `SH-22-add-bounded-worker-pool`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `priority:critical`  
**Depends on:** SH-19, SH-20, SH-21

**What:** Execute scheduled requests with a fixed-capacity worker pool and clear
ownership of channels and cancellation.

**Why:** Unbounded goroutines, blocked sends, or unclear shutdown would make the
diagnostic client itself unreliable.

**Suggested approach:** Use a parent context, one job channel, explicit producer
closure, and result collection that can drain after cancellation.

**Acceptance criteria:**

- [ ] In-flight requests never exceed the configured worker limit.
- [ ] No work is started after cancellation or request-budget exhaustion.
- [ ] Workers, scheduler, and collector terminate on success and every failure
  path.
- [ ] Leak-focused and race-detector tests pass.

## SH-23: Separate warm-up lifecycle and evidence

**Branch:** `SH-23-handle-warmup-requests`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`  
**Depends on:** SH-19, SH-21, SH-22

**What:** Run configured warm-ups before applicable measured stages while keeping
their outcomes visible but excluded from primary statistics.

**Why:** Warm-ups reduce startup artifacts but can also fail, mutate state, and
consume real traffic.

**Suggested approach:** Use the same request executor, limiter, safety budget,
and cancellation path with an explicit sample role.

**Acceptance criteria:**

- [ ] Warm-ups count toward total requests and aggregate rate limits.
- [ ] Their latency is excluded from measured percentiles and throughput.
- [ ] Warm-up failures are surfaced and policy for continuing is documented.
- [ ] Unsafe-method warnings include warm-up repetitions.

## SH-24: Calculate exact latency statistics

**Branch:** `SH-24-calculate-latency-statistics`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:metrics`, `priority:critical`  
**Depends on:** SH-19

**What:** Implement min, max, mean, median, p90, p95, p99, and standard deviation
over bounded exact samples.

**Why:** Statistical definitions must be deterministic across diagnosis, export,
comparison, and tests.

**Suggested approach:** Ratify D-005, define empty/singleton behavior, use stable
duration arithmetic, and document population versus sample deviation.

**Acceptance criteria:**

- [ ] Percentiles use documented `(n-1)*p` linear interpolation.
- [ ] Empty, singleton, duplicate, ordered, unordered, and extreme inputs work.
- [ ] Overflow and invalid negative durations are rejected or handled explicitly.
- [ ] Tests use independently calculated golden cases.

## SH-25: Aggregate outcomes, throughput, and concurrency

**Branch:** `SH-25-aggregate-experiment-metrics`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:metrics`, `priority:critical`  
**Depends on:** SH-19, SH-24

**What:** Aggregate result counts, status classes, timeouts, bytes, throughput,
scheduling delay, actual concurrency, protocol, and connection reuse.

**Why:** Diagnostic rules need achieved behavior, not only configured settings.

**Suggested approach:** Derive metrics from immutable samples after execution;
avoid a heavily locked hot-path aggregator until profiling justifies one.

**Acceptance criteria:**

- [ ] Attempted, completed, successful, HTTP error, transport error, timeout, and
  cancelled counts reconcile.
- [ ] Offered and achieved rates use documented denominators.
- [ ] Peak and time-weighted actual in-flight concurrency are available.
- [ ] Empty and interrupted experiments return valid, explicitly limited results.

## SH-26: Run baseline sequential experiments

**Branch:** `SH-26-add-baseline-experiment`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `area:metrics`, `priority:high`  
**Depends on:** SH-21, SH-22, SH-23, SH-25

**What:** Implement the sequential baseline experiment with warm-up and measured
phases.

**Why:** Baseline establishes normal behavior and anchors every later diagnostic
comparison.

**Suggested approach:** Reuse the common runner at concurrency one and retain
both total and phase samples.

**Acceptance criteria:**

- [ ] The configured measured request count is attempted unless cancelled or a
  documented stop policy applies.
- [ ] Baseline result contains latency, phase, outcome, rate, and reuse metrics.
- [ ] Warm-ups remain excluded from aggregates.
- [ ] Local integration tests cover success, mixed errors, timeout, and cancellation.

## SH-27: Compare fresh and reused connections fairly

**Branch:** `SH-27-add-connection-experiment`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `area:http`, `priority:high`, `needs-decision`  
**Depends on:** SH-13, SH-23, SH-25

**What:** Implement comparable fresh-connection and reused-connection arms.

**Why:** Connection setup conclusions are only credible when transport state,
load, ordering, and protocol observations are controlled.

**Suggested approach:** Ratify D-018; use isolated transports, a recorded seeded
alternation order, and evidence counts for truly new/reused connections.

**Acceptance criteria:**

- [ ] Arms share request config, rate, timeout, and measured sample target.
- [ ] Transport pools never cross arms and are closed after the experiment.
- [ ] Reported comparisons use observed connection state, not requested mode alone.
- [ ] HTTP/2 and proxy caveats can produce insufficient evidence rather than a
  false conclusion.

## SH-28: Generate safe concurrency stages

**Branch:** `SH-28-generate-concurrency-stages`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`  
**Depends on:** SH-02, SH-04

**What:** Generate ascending, deduplicated stages from the configured maximum.

**Why:** The PRD defines defaults of 1, 5, and 10 but needs deterministic behavior
for smaller, intermediate, and larger overrides.

**Suggested approach:** Specify a simple stage-selection algorithm that always
includes one and the requested maximum without exceeding safety ceilings.

**Acceptance criteria:**

- [ ] Defaults yield exactly `[1, 5, 10]`.
- [ ] Maximums below, between, and above defaults produce documented stages.
- [ ] Stages are positive, ascending, unique, and capped.
- [ ] Unit tests cover representative and boundary maximums.

## SH-29: Run concurrency sensitivity experiments

**Branch:** `SH-29-add-concurrency-experiment`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:feature`, `area:runner`, `area:metrics`, `priority:high`  
**Depends on:** SH-22, SH-23, SH-25, SH-28

**What:** Execute one measured experiment per concurrency stage under the shared
aggregate rate.

**Why:** Sloth needs evidence about latency, errors, throughput, and actual load
as in-flight capacity changes.

**Suggested approach:** Reuse the common scheduler and workers, reset only
stage-local aggregation, and record offered versus achieved conditions.

**Acceptance criteria:**

- [ ] Each stage has its own complete config and aggregate result.
- [ ] Increasing workers alone does not alter the configured aggregate rate.
- [ ] Actual in-flight concurrency and scheduling delays are recorded.
- [ ] Cancellation returns ordered partial stage results without leaks.

## SH-30: Prove runner cancellation and race safety

**Branch:** `SH-30-test-runner-concurrency`  
**Milestone:** M3 Runner & Metrics  
**Labels:** `type:test`, `area:runner`, `priority:critical`  
**Depends on:** SH-20, SH-21, SH-22, SH-23, SH-29

**What:** Build a focused concurrency test suite for lifecycle, bounds, and data
ownership.

**Why:** Happy-path metrics are irrelevant if cancellation leaks workers or
concurrent collection races.

**Suggested approach:** Combine controllable fake executors, blocked servers,
high-iteration tests, goroutine accounting, and `go test -race`.

**Acceptance criteria:**

- [ ] Tests cover cancellation before start, during limiter wait, in flight, and
  during result collection.
- [ ] Worker and request bounds hold under slow and failing executors.
- [ ] Channel closure and partial-result paths do not panic or deadlock.
- [ ] The package passes repeated race-detector runs in CI.

## SH-31: Define findings, thresholds, and rule registry

**Branch:** `SH-31-create-diagnostic-engine`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `priority:critical`  
**Depends on:** SH-19, SH-25

**What:** Create the pure diagnostic engine contract, finding schema, centralized
threshold set, and deterministic rule registry.

**Why:** Rules must be testable without HTTP or CLI code and reproducible for a
known ruleset version.

**Suggested approach:** Accept structured experiment results and return ordered
findings with typed evidence; inject a versioned threshold profile.

**Acceptance criteria:**

- [ ] Findings contain code, severity, confidence, title, observation, possible
  causes, recommendation, typed evidence, and rule version.
- [ ] Rules have no network, filesystem, clock, or renderer dependencies.
- [ ] Thresholds are centralized and documented with units.
- [ ] Finding order is stable for identical input.

## SH-32: Classify evidence strength and contradictions

**Branch:** `SH-32-score-diagnostic-evidence`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `priority:high`  
**Depends on:** SH-31

**What:** Define reusable minimum-sample, completeness, consistency, and
contradiction handling for qualitative confidence.

**Why:** Small or confounded samples must weaken or suppress claims consistently
across rules.

**Suggested approach:** Return an evidence assessment with confidence and reasons;
do not assign probability-like numeric confidence.

**Acceptance criteria:**

- [ ] Confidence values and evidence requirements are documented.
- [ ] Sparse, missing, contradictory, and interrupted data can downgrade or
  suppress findings.
- [ ] p99 never receives high-confidence treatment from the default 100 samples.
- [ ] Boundary tests cover every confidence transition.

## SH-33: Detect elevated first-byte wait (D001)

**Branch:** `SH-33-detect-first-byte-wait`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`  
**Depends on:** SH-31, SH-32

**What:** Implement the `HIGH_FIRST_BYTE_WAIT` rule using available first-byte
share evidence.

**Why:** Dominant pre-first-byte time is a common actionable observation, but it
does not prove server execution is slow.

**Suggested approach:** Evaluate robust central tendency across valid samples and
include availability, variability, and outcome checks.

**Acceptance criteria:**

- [ ] The finding reports sample count and first-byte share evidence.
- [ ] Possible causes include processing, dependencies, queueing, and network delay.
- [ ] Missing phase data or weak evidence yields no claim or D006.
- [ ] Positive, negative, threshold, contradictory, and sparse tests pass.

## SH-34: Detect connection setup overhead (D002)

**Branch:** `SH-34-detect-connection-overhead`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `area:http`  
**Depends on:** SH-27, SH-31, SH-32

**What:** Compare observed fresh and reused connection latency and phase evidence.

**Why:** Users need to know when TCP/TLS setup is material, without confusing
protocol or proxy behavior with a controlled comparison.

**Suggested approach:** Require comparable arms and enough observed new/reused
connections; use both absolute and relative thresholds.

**Acceptance criteria:**

- [ ] Evidence includes arm sizes, observed reuse counts, latency delta, and
  available TCP/TLS contribution.
- [ ] HTTP/2 multiplexing and proxy ambiguity can reduce confidence.
- [ ] Recommendation covers keep-alive and pooling without asserting server cause.
- [ ] Full rule boundary matrix passes.

## SH-35: Detect tail-latency variability (D003)

**Branch:** `SH-35-detect-tail-variability`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `area:metrics`  
**Depends on:** SH-24, SH-31, SH-32

**What:** Identify a materially elevated upper tail relative to median behavior.

**Why:** Averages can hide intermittent delays that dominate user experience.

**Suggested approach:** Combine p95/p50 ratio, absolute gap, spread, and sample
adequacy so low-latency jitter does not trigger noisy findings.

**Acceptance criteria:**

- [ ] Evidence includes p50, p95, p99 caveat, ratio, absolute gap, and count.
- [ ] Both relative and absolute thresholds must be satisfied where documented.
- [ ] Small samples and isolated invalid outcomes reduce confidence.
- [ ] Full rule boundary matrix passes.

## SH-36: Detect concurrency sensitivity (D004)

**Branch:** `SH-36-detect-concurrency-sensitivity`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `area:runner`, `priority:high`  
**Depends on:** SH-29, SH-31, SH-32

**What:** Evaluate latency, throughput, errors, and scheduling as achieved load
changes between stages.

**Why:** Configuring more workers is not evidence of saturation if offered rate
or actual in-flight requests did not materially increase.

**Suggested approach:** Establish a load-change eligibility check before applying
latency/throughput deterioration thresholds.

**Acceptance criteria:**

- [ ] No saturation-style finding occurs without meaningful achieved-load change.
- [ ] Evidence includes offered rate, achieved rate, actual concurrency, p50/p95,
  successes, errors, and scheduling delay by stage.
- [ ] Client-side scheduling pressure is distinguished from endpoint behavior.
- [ ] Full rule boundary and confounding matrix passes.

## SH-37: Detect elevated errors and timeouts (D005)

**Branch:** `SH-37-detect-error-rates`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`  
**Depends on:** SH-25, SH-31, SH-32

**What:** Detect material timeout, transport, and HTTP server-error rates while
preserving their distinct distributions.

**Why:** Different failure modes imply different follow-up investigations and
must not be collapsed into one count.

**Suggested approach:** Evaluate absolute counts and rates, distinguish 4xx from
5xx, and account for cancellation and truncated runs.

**Acceptance criteria:**

- [ ] Findings identify which outcome classes crossed thresholds.
- [ ] User cancellation is excluded from service-instability evidence.
- [ ] 4xx and 5xx policies are documented separately.
- [ ] Full rule boundary matrix passes.

## SH-38: Explain insufficient evidence (D006)

**Branch:** `SH-38-report-insufficient-evidence`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:diagnosis`, `priority:critical`  
**Depends on:** SH-31, SH-32, SH-33, SH-34, SH-35, SH-36, SH-37

**What:** Emit a structured inconclusive finding when the suite cannot support a
defensible diagnostic claim.

**Why:** Honest uncertainty is a required product feature, not a generic error or
an empty report.

**Suggested approach:** Aggregate suppression and evidence-assessment reasons;
avoid D006 when strong positive or clearly normal evidence already explains the
result.

**Acceptance criteria:**

- [ ] D006 names concrete reasons such as sparse samples, unavailable phases,
  unchanged load, incompatible arms, or interruption.
- [ ] It offers a safe next measurement when one is available.
- [ ] It never fabricates a likely cause.
- [ ] Tests cover sparse, conflicting, interrupted, and fully adequate inputs.

## SH-39: Orchestrate the standard diagnose suite

**Branch:** `SH-39-add-diagnose-command`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:cli`, `area:runner`, `priority:critical`  
**Depends on:** SH-26, SH-27, SH-29, SH-33, SH-34, SH-35, SH-36, SH-37, SH-38

**What:** Wire baseline, connection, concurrency, aggregation, and rule evaluation
into `sloth diagnose`.

**Why:** This is Sloth's defining end-to-end workflow.

**Suggested approach:** Build an explicit suite plan, display its budget, execute
stages under one limiter/context, then evaluate complete or partial evidence.

**Acceptance criteria:**

- [ ] Defaults and overrides match the PRD and effective config is displayed.
- [ ] The complete planned request count and estimated minimum duration are shown
  before traffic starts.
- [ ] Ctrl+C yields prompt shutdown and a defined exit status.
- [ ] Partial failures remain visible and can produce D006 rather than a crash.

## SH-40: Render diagnostic progress and consolidated findings

**Branch:** `SH-40-render-diagnostic-report`  
**Milestone:** M4 Diagnosis  
**Labels:** `type:feature`, `area:cli`, `area:diagnosis`  
**Depends on:** SH-05, SH-39

**What:** Add terminal progress, experiment summaries, findings, limitations, and
next-step rendering.

**Why:** Long default runs need feedback, while the final report must stay concise
and careful about causality.

**Suggested approach:** Render from view models; enable transient progress only on
interactive terminals and send it to stderr.

**Acceptance criteria:**

- [ ] Baseline and concurrency tables include counts, p50/p95/p99, errors, offered
  and achieved load, and actual concurrency where relevant.
- [ ] Findings show observation, possible causes, recommendation, confidence, and
  evidence without overstating root cause.
- [ ] Noninteractive output has no animation or control sequences.
- [ ] Snapshot tests cover success, mixed findings, D006, and narrow terminals.

## SH-41: Define the canonical v1 report model

**Branch:** `SH-41-model-json-reports`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `priority:critical`  
**Depends on:** SH-06, SH-19, SH-25, SH-31

**What:** Define the versioned in-memory report with run, target, config,
environment, overall, per-experiment, diagnostics, and limitations sections.

**Why:** Export and comparison need one stable contract that resolves the PRD's
abbreviated and ambiguous overall request counts.

**Suggested approach:** Keep explicit units in field names, use null for unavailable
values, and separate all-suite totals from experiment aggregates.

**Acceptance criteria:**

- [ ] Schema, tool, and diagnostic-rule versions are distinct.
- [ ] Run ID, UTC timestamp, redacted target/fingerprint, effective config, and
  bounded environment metadata are present.
- [ ] Overall counts reconcile across experiments without mixing warm-up latency.
- [ ] No raw secrets, request bodies, or response bodies are representable in the
  default report model.

## SH-42: Publish and validate the v1 JSON Schema

**Branch:** `SH-42-add-report-json-schema`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `type:docs`, `priority:critical`  
**Depends on:** SH-41

**What:** Add `docs/schema/report-v1.schema.json` and automated model/schema
conformance tests.

**Why:** Machine consumers need a precise, reviewable contract beyond Go structs.

**Suggested approach:** Use JSON Schema with explicit required fields, units,
enums, nullability, numeric bounds, and forward-compatible extension policy.

**Acceptance criteria:**

- [ ] Valid full, aggregate-only, partial, and interrupted reports pass schema
  validation.
- [ ] Missing required fields, invalid units/ranges, and unbounded sample arrays
  fail validation.
- [ ] Sample reports are validated in CI.
- [ ] Schema evolution rules are documented.

## SH-43: Parse reports with version-aware validation

**Branch:** `SH-43-validate-saved-reports`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `priority:critical`  
**Depends on:** SH-42

**What:** Read bounded JSON files, validate schema/version, and return typed
reports with actionable errors.

**Why:** Compare must reject malformed or unsupported evidence rather than
silently treating missing data as zero.

**Suggested approach:** Cap input size, disallow trailing garbage, preserve
unknown forward fields, and separate unsupported-major from newer-minor handling.

**Acceptance criteria:**

- [ ] Missing, unreadable, oversized, malformed, truncated, and unsupported
  reports produce distinct errors.
- [ ] Unknown compatible fields do not break parsing.
- [ ] Null/unavailable values remain distinct from numeric zero.
- [ ] Parser fuzz tests cannot panic or allocate beyond documented bounds.

## SH-44: Resolve paths and write reports atomically

**Branch:** `SH-44-write-reports-atomically`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `area:security`, `priority:high`  
**Depends on:** SH-41

**What:** Implement Desktop default resolution, explicit output paths, collision
handling, `--force`, and atomic writes.

**Why:** Export must not corrupt or silently overwrite evidence, and Desktop may
not exist on every system.

**Suggested approach:** Resolve home with platform APIs, create a temporary file
in the destination directory, sync/close as appropriate, then rename.

**Acceptance criteria:**

- [ ] Default names include UTC/local-safe timestamp plus collision-resistant
  suffix where needed.
- [ ] Missing or unwritable Desktop produces an actionable error and never creates
  it silently.
- [ ] Relative paths resolve against the working directory.
- [ ] Existing targets require `--force`; failed writes leave no partial final file.

## SH-45: Implement the export command

**Branch:** `SH-45-add-export-command`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:cli`, `area:report`, `priority:high`  
**Depends on:** SH-39, SH-42, SH-44

**What:** Run the standard suite and persist a validated report through
`sloth export`.

**Why:** Saved evidence enables before/after workflows and CI baselines.

**Suggested approach:** Reuse diagnose orchestration, construct the canonical
report, validate before write, and print only the final path to stdout when
appropriate.

**Acceptance criteria:**

- [ ] Default and custom output paths work on supported platforms.
- [ ] Every successfully written file validates against the bundled schema.
- [ ] Progress and warnings do not enter the JSON file or machine stdout.
- [ ] Cancellation or validation failure does not leave a final report file.

## SH-46: Support bounded optional sample export

**Branch:** `SH-46-export-bounded-samples`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `area:security`  
**Depends on:** SH-41, SH-42, SH-45

**What:** Add `--include-samples` with a documented maximum and explicit sample
schema.

**Why:** Detailed evidence enables offline analysis and uncertainty estimates,
but unbounded arrays threaten privacy, memory, and file size.

**Suggested approach:** Store only timing/outcome metadata already approved for
the request sample; enforce count and output-size ceilings before writing.

**Acceptance criteria:**

- [ ] Samples are absent by default.
- [ ] Included samples never contain raw URLs, headers, or bodies.
- [ ] Count and serialized-size limits produce actionable errors or documented
  truncation metadata.
- [ ] Schema and round-trip tests cover reports with and without samples.

## SH-47: Classify report compatibility

**Branch:** `SH-47-check-report-compatibility`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:report`, `area:metrics`, `priority:critical`, `needs-decision`  
**Depends on:** SH-43

**What:** Produce a field-by-field compatibility result before comparison.

**Why:** Performance deltas are misleading when targets, methods, timeouts,
rates, stages, or experiment semantics differ.

**Suggested approach:** Ratify D-017 and return fatal mismatches, warnings, and
compatible experiment pairs as structured data.

**Acceptance criteria:**

- [ ] Schema, target, method, request policy, timeout, rate, stages, experiment
  kinds, sample sizes, and rule semantics are evaluated.
- [ ] Gate mode rejects every material mismatch.
- [ ] Descriptive mode can compare compatible subsets with prominent warnings.
- [ ] Compatibility tests cover each field independently and in combination.

## SH-48: Calculate safe report deltas

**Branch:** `SH-48-calculate-report-deltas`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:metrics`, `area:report`, `priority:high`  
**Depends on:** SH-24, SH-47

**What:** Compute absolute and percentage changes for latency, throughput,
outcomes, scheduling, connection, and experiment-specific metrics.

**Why:** All renderers and gates need consistent sign, units, rounding, and
zero-baseline behavior.

**Suggested approach:** Use typed direction metadata so lower-is-better latency
and higher-is-better throughput cannot be interpreted by string formatting.

**Acceptance criteria:**

- [ ] p50/p90/p95/p99, mean, errors, timeouts, throughput, and available phase
  metrics support absolute deltas.
- [ ] Percentage change is undefined rather than infinite when baseline is zero.
- [ ] Missing or incompatible metrics do not become zeros.
- [ ] Rounding affects display only, not gate decisions.

## SH-49: Render before-and-after comparisons

**Branch:** `SH-49-render-comparison-table`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:cli`, `area:report`  
**Depends on:** SH-47, SH-48

**What:** Add the terminal comparison table, compatibility warnings, uncertainty
notes, and careful overall result.

**Why:** Users need a fast view of what changed without hiding incompatible or
missing evidence.

**Suggested approach:** Render common top-level metrics first and per-experiment
details second; use explicit `N/A` and neutral wording for uncertain changes.

**Acceptance criteria:**

- [ ] Table shows before, after, absolute change, and percentage when defined.
- [ ] Improvements/regressions use correct metric direction and accessible signs,
  not color alone.
- [ ] Material compatibility warnings appear before any conclusion.
- [ ] Snapshot tests cover improvement, regression, mixed, zero, and incompatible cases.

## SH-50: Add compare command and regression gate

**Branch:** `SH-50-add-compare-command`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:feature`, `area:cli`, `area:report`, `priority:critical`  
**Depends on:** SH-05, SH-43, SH-47, SH-48, SH-49

**What:** Implement `sloth compare` plus `--fail-on-regression` for p50, p95, or
p99.

**Why:** CI needs a stable decision and exit status, not only a visual table.

**Suggested approach:** Validate both reports and gate eligibility before applying
the unrounded configured percentage threshold.

**Acceptance criteria:**

- [ ] Relative and absolute file paths work.
- [ ] Default gate metric is p95 and allowed metrics are validated.
- [ ] Incompatible or inadequate evidence fails validation rather than passing.
- [ ] Regression, no regression, malformed report, and interruption use the
  documented exit codes.

## SH-51: Add report and comparison fixture suite

**Branch:** `SH-51-test-report-workflows`  
**Milestone:** M5 Export & Compare  
**Labels:** `type:test`, `area:report`, `priority:high`  
**Depends on:** SH-42, SH-43, SH-45, SH-46, SH-50

**What:** Maintain deterministic golden reports for schema, export, compatibility,
comparison, and gate behavior.

**Why:** Versioned data contracts require durable examples and regression tests.

**Suggested approach:** Generate fixtures from fixed domain values, validate all
committed JSON in CI, and reserve migration tests for future schema versions.

**Acceptance criteria:**

- [ ] Fixtures cover aggregate-only, samples, partial, error-heavy, and
  incompatible reports.
- [ ] Golden output has stable ordering and UTC timestamps supplied by a fake clock.
- [ ] CI validates fixtures against the JSON Schema and typed parser.
- [ ] A fixture update requires an explicit schema or presentation rationale.

## SH-52: Harden terminal accessibility and noninteractive use

**Branch:** `SH-52-harden-terminal-output`  
**Milestone:** M6 Hardening  
**Labels:** `type:feature`, `area:cli`  
**Depends on:** SH-17, SH-40, SH-49

**What:** Apply consistent color, Unicode, width, TTY, redirection, and
`NO_COLOR` behavior across all commands.

**Why:** Sloth must remain understandable in narrow terminals, scripts, CI logs,
and environments without color or Unicode support.

**Suggested approach:** Centralize terminal capability detection and render tokens;
test explicit flags before environment and automatic behavior.

**Acceptance criteria:**

- [ ] `NO_COLOR` and `--no-color` disable ANSI output everywhere.
- [ ] Redirected/noninteractive output contains no animation or cursor controls.
- [ ] ASCII fallback conveys every status without relying on glyphs or color.
- [ ] Representative 80-, 60-, and 40-column snapshots remain readable.

## SH-53: Audit and test privacy boundaries

**Branch:** `SH-53-audit-data-redaction`  
**Milestone:** M6 Hardening  
**Labels:** `type:test`, `area:security`, `priority:critical`  
**Depends on:** SH-09, SH-10, SH-41, SH-46

**What:** Verify that secrets and bodies cannot escape through output, reports,
errors, debug formatting, fixtures, or panics.

**Why:** Diagnostic targets commonly contain credentials and sensitive request
data.

**Suggested approach:** Seed canary secrets into every input surface and assert
their absence from every serialized or rendered output and wrapped error.

**Acceptance criteria:**

- [ ] Authorization/cookie headers, URL credentials, secret query values, and body
  content are covered by canary tests.
- [ ] Redirect and transport errors cannot reveal raw sensitive targets.
- [ ] Reports and samples expose only the documented redacted identity.
- [ ] A short threat model documents trust boundaries and residual risks.

## SH-54: Add fuzzing and malformed-input resilience

**Branch:** `SH-54-fuzz-input-boundaries`  
**Milestone:** M6 Hardening  
**Labels:** `type:test`, `area:security`  
**Depends on:** SH-03, SH-09, SH-43, SH-47

**What:** Fuzz URL/header parsing, report parsing, schema compatibility, and
comparison calculations.

**Why:** User-controlled inputs should not cause panics, hangs, unbounded
allocation, or secret-bearing errors.

**Suggested approach:** Add seeded Go fuzz targets with size caps and retain every
discovered regression as a deterministic corpus case.

**Acceptance criteria:**

- [ ] Fuzz targets enforce input and runtime bounds.
- [ ] Parsers and comparison code do not panic on arbitrary bytes.
- [ ] Known edge cases remain in the corpus.
- [ ] A documented short fuzz run is available locally and in scheduled CI.

## SH-55: Complete network failure integration tests

**Branch:** `SH-55-test-network-failures`  
**Milestone:** M6 Hardening  
**Labels:** `type:test`, `area:http`, `area:runner`, `priority:high`  
**Depends on:** SH-18, SH-30, SH-39

**What:** Exercise DNS, refused connection, TLS, resets, timeouts, partial bodies,
redirect loops, proxy behavior, and interruption end to end.

**Why:** Failure paths define diagnostic trustworthiness and are where lifecycle
bugs are most likely.

**Suggested approach:** Prefer local listeners, custom resolvers/dialers, test
certificates, and deterministic fault injection over third-party endpoints.

**Acceptance criteria:**

- [ ] Each documented transport category is reproduced by a local test.
- [ ] Commands return the expected redacted error, partial evidence, and exit code.
- [ ] Cancellation latency stays within a generous documented bound.
- [ ] The full suite passes under the race detector.

## SH-56: Benchmark Sloth's measurement overhead

**Branch:** `SH-56-benchmark-client-overhead`  
**Milestone:** M6 Hardening  
**Labels:** `type:research`, `area:metrics`, `priority:high`  
**Depends on:** SH-18, SH-30, SH-39

**What:** Measure allocation, CPU, instrumentation, scheduling, and memory
overhead within supported operating bounds.

**Why:** Sloth cannot claim minimal measurement impact without reproducible
evidence separating client overhead from endpoint latency.

**Suggested approach:** Add Go benchmarks and a loopback methodology comparing
instrumented and minimally instrumented clients across representative modes.

**Acceptance criteria:**

- [ ] Benchmarks report allocations and execution cost for trace, aggregation,
  rendering-free runner, and report serialization paths.
- [ ] Scheduler accuracy and memory growth are tested at normal safety ceilings.
- [ ] Results include hardware/OS/Go metadata and reproducible commands.
- [ ] Documentation avoids unsupported accuracy or superiority claims.

## SH-57: Add deterministic sample API and examples

**Branch:** `SH-57-add-sample-api`  
**Milestone:** M6 Hardening  
**Labels:** `type:docs`, `type:test`, `area:http`  
**Depends on:** SH-39, SH-45, SH-50

**What:** Provide a small local example server with controllable fast, delayed
header, slow stream, error, and concurrency-sensitive routes.

**Why:** Users and end-to-end tests need reproducible targets without external
services. This is the appropriate place for a basic server; it is not Sloth
runtime infrastructure.

**Suggested approach:** Keep the sample isolated under `examples/sample-api`,
dependency-light, clearly unsafe for production, and configurable by flags.

**Acceptance criteria:**

- [ ] The sample starts with one documented command and prints its local URL.
- [ ] Routes deterministically exercise inspect, diagnose, export, and compare.
- [ ] It binds to loopback by default and has clean shutdown.
- [ ] End-to-end smoke tests use the sample without fixed ports.

## SH-58: Complete architecture and methodology documentation

**Branch:** `SH-58-complete-technical-docs`  
**Milestone:** M6 Hardening  
**Labels:** `type:docs`, `priority:high`  
**Depends on:** SH-12, SH-24, SH-31, SH-42, SH-47, SH-56

**What:** Complete architecture, commands, methodology, diagnostics,
benchmarking, schema, and examples documentation.

**Why:** Timing definitions and limitations are part of product correctness, not
optional explanation.

**Suggested approach:** Link each public behavior to its source issue and tests;
include diagrams only where they clarify lifecycle or ownership.

**Acceptance criteria:**

- [ ] Every command and flag has syntax, default, safety behavior, and examples.
- [ ] Every timing/statistical definition, diagnostic threshold, false positive,
  and coordinated-omission limitation is documented.
- [ ] Architecture covers ownership, data flow, cancellation, and package boundaries.
- [ ] Benchmarking instructions reproduce published evidence.

## SH-59: Validate the supported platform matrix

**Branch:** `SH-59-test-supported-platforms`  
**Milestone:** M7 v1 Release  
**Labels:** `type:chore`, `area:release`, `priority:high`  
**Depends on:** SH-07, SH-44, SH-52, SH-55

**What:** Build and smoke-test macOS ARM64/AMD64, Linux ARM64/AMD64, and Windows
AMD64 behavior.

**Why:** Paths, atomic rename, signals, terminal detection, and executable naming
differ across platforms.

**Suggested approach:** Cross-build every target and run native smoke tests on
available GitHub-hosted runners; document gaps explicitly.

**Acceptance criteria:**

- [ ] All five target binaries build from a clean checkout.
- [ ] Native CI verifies help, version, inspect against a local endpoint, and
  report path behavior on macOS, Linux, and Windows.
- [ ] Platform-specific atomicity or signal limitations are documented.
- [ ] No release target depends on CGO unless explicitly justified.

## SH-60: Package releases with GoReleaser

**Branch:** `SH-60-package-release-artifacts`  
**Milestone:** M7 v1 Release  
**Labels:** `type:chore`, `area:release`, `priority:critical`  
**Depends on:** SH-06, SH-59

**What:** Configure tagged builds, archives, checksums, changelog, and GitHub
Release publication.

**Why:** Users without Go need verifiable standalone binaries.

**Suggested approach:** Pin GoReleaser behavior, inject version metadata, use
least-privilege release permissions, and test a snapshot build before tags.

**Acceptance criteria:**

- [ ] Snapshot release produces all supported archives and checksum files.
- [ ] Archive naming and contained binary names are consistent per platform.
- [ ] Release binaries report the tag version and commit.
- [ ] Installation and checksum verification steps are documented.

## SH-61: Create the real terminal demo and final README

**Branch:** `SH-61-finish-readme-demo`  
**Milestone:** M7 v1 Release  
**Labels:** `type:docs`, `area:release`  
**Depends on:** SH-57, SH-58, SH-60

**What:** Produce a truthful terminal recording and a README understandable in
under one minute.

**Why:** The project should communicate its differentiator and limitations using
real behavior, not mocked performance numbers.

**Suggested approach:** Record the deterministic sample API workflow and keep the
README focused on install, quick start, diagnose, export/compare, architecture,
limitations, development, and license.

**Acceptance criteria:**

- [ ] The recording uses a released or release-candidate binary and reproducible
  local sample endpoint.
- [ ] Displayed metrics are genuine and labeled illustrative where appropriate.
- [ ] All commands in the README are exercised by a docs smoke check where feasible.
- [ ] README links to detailed methodology rather than duplicating it.

## SH-62: Execute the v1 release readiness review

**Branch:** `SH-62-verify-v1-readiness`  
**Milestone:** M7 v1 Release  
**Labels:** `type:chore`, `area:release`, `priority:critical`  
**Depends on:** SH-53, SH-54, SH-55, SH-56, SH-58, SH-59, SH-60, SH-61

**What:** Verify every PRD success criterion and publish the evidence used for the
v1 go/no-go decision.

**Why:** “Feature complete” should mean demonstrated behavior, not merely closed
issues.

**Suggested approach:** Maintain a checklist mapping each criterion to tests,
docs, artifacts, and any accepted limitations; run from a clean clone.

**Acceptance criteria:**

- [ ] All required commands work from an installed release artifact.
- [ ] Unit, integration, end-to-end, race, schema, security, and platform checks pass.
- [ ] Six diagnostic rules and their evidence matrices are documented and verified.
- [ ] Known limitations and deferred work are explicit.
- [ ] Release notes contain no fabricated performance, accuracy, or adoption claims.

## SH-63: Add no-network diagnostic dry run

**Branch:** `SH-63-preview-diagnostic-plan`  
**Milestone:** Post-v1  
**Labels:** `type:feature`, `area:cli`, `area:security`, `post-v1`  
**Depends on:** SH-04, SH-28, SH-39

**What:** Add `diagnose --dry-run` to print the effective experiment plan, traffic
budget, and estimated minimum duration without sending requests.

**Why:** The default suite can contain hundreds of requests; previewing it improves
safety and CI predictability.

**Suggested approach:** Reuse the same plan builder and validation path as live
execution, stopping before transport creation.

**Acceptance criteria:**

- [ ] Dry run performs no DNS lookup, connection, or output file write.
- [ ] It lists stages, warm-ups, measured requests, rate, concurrency, and total budget.
- [ ] Human and JSON plan output are available.
- [ ] The preview exactly matches the plan a live run would execute.

## SH-64: Re-analyze saved reports offline

**Branch:** `SH-64-add-offline-analysis`  
**Milestone:** Post-v1  
**Labels:** `type:feature`, `area:diagnosis`, `area:report`, `post-v1`  
**Depends on:** SH-31, SH-43, SH-46

**What:** Add `sloth analyze <report.json>` to run a compatible newer ruleset
against saved evidence.

**Why:** Diagnostic logic can improve without forcing users to contact an endpoint
again, when the report retained sufficient evidence.

**Suggested approach:** Validate evidence completeness, preserve original findings,
and label re-analysis time and rule version separately from measurement time.

**Acceptance criteria:**

- [ ] The command never performs network traffic.
- [ ] Original and recomputed findings remain distinguishable.
- [ ] Aggregate-only reports produce D006 when samples required by a rule are absent.
- [ ] Output records schema and ruleset compatibility decisions.

## SH-65: Compare repeated runs with uncertainty

**Branch:** `SH-65-compare-repeated-runs`  
**Milestone:** Post-v1  
**Labels:** `type:feature`, `area:metrics`, `area:report`, `post-v1`, `type:research`  
**Depends on:** SH-46, SH-47, SH-48, SH-50

**What:** Compare groups of compatible runs and quantify between-run variability
or bootstrap intervals where evidence permits.

**Why:** One noisy before/after pair should not automatically become a strong
performance claim.

**Suggested approach:** Start with repeated-run aggregation and a documented
resampling method over compatible samples; expose limitations prominently.

**Acceptance criteria:**

- [ ] Group compatibility is validated before analysis.
- [ ] Method, resample count/seed, interval definition, and minimum evidence are documented.
- [ ] Deterministic synthetic datasets validate interval behavior.
- [ ] Insufficient evidence never yields a significance claim.

## SH-66: Gate phase-level regressions

**Branch:** `SH-66-gate-phase-regressions`  
**Milestone:** Post-v1  
**Labels:** `type:feature`, `area:metrics`, `area:report`, `post-v1`  
**Depends on:** SH-48, SH-50, SH-65

**What:** Extend comparison and gates to DNS, connect, TLS, first-byte, body,
and scheduling metrics when both reports contain compatible evidence.

**Why:** Total latency can hide which observable phase changed after an optimization.

**Suggested approach:** Reuse typed metric direction and compatibility rules;
require adequate availability per selected phase.

**Acceptance criteria:**

- [ ] Users can select documented phase metrics for display and gates.
- [ ] N/A-heavy or semantically incompatible phases fail gate validation.
- [ ] Output states the phase boundary and evidence coverage.
- [ ] Zero and missing baselines follow the safe delta contract.

## SH-67: Export portable CI summaries

**Branch:** `SH-67-add-ci-summary-format`  
**Milestone:** Post-v1  
**Labels:** `type:feature`, `area:report`, `post-v1`  
**Depends on:** SH-50, SH-58

**What:** Add one vendor-neutral concise CI result format, such as JUnit XML,
after the JSON contract stabilizes.

**Why:** Teams should be able to surface regression results in CI without parsing
terminal tables or binding Sloth to one provider.

**Suggested approach:** Map validation failures and regression gates to test cases
while retaining the JSON report as the detailed evidence artifact.

**Acceptance criteria:**

- [ ] The format validates against its standard schema where available.
- [ ] Regression, validation failure, and success map unambiguously.
- [ ] No secrets or ANSI sequences are emitted.
- [ ] GitHub Actions usage is documented without making GitHub a runtime dependency.

## SH-68: Publish a Homebrew formula

**Branch:** `SH-68-publish-homebrew-formula`  
**Milestone:** Post-v1  
**Labels:** `type:chore`, `area:release`, `post-v1`  
**Depends on:** SH-60, SH-62

**What:** Automate Homebrew installation after release artifact names and checksums
are stable.

**Why:** macOS users benefit from a familiar upgrade path, but formula maintenance
should not block the first reliable release process.

**Suggested approach:** Use GoReleaser's supported formula workflow and a documented
tap/repository decision with least-privilege credentials.

**Acceptance criteria:**

- [ ] `brew install` installs a checksum-verified supported macOS binary.
- [ ] `sloth --version` matches the formula version.
- [ ] Upgrade and uninstall paths are tested.
- [ ] Release failure cannot publish a formula pointing to missing artifacts.
