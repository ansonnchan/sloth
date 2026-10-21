# Decision log

This log refines the PRD into implementation-level contracts. `Accepted` means
the roadmap assumes the decision; changing it requires updating dependent issues
and documentation. `Proposed` means the named design issue must ratify it before
implementation depends on it.

## Accepted decisions

### D-001 — Sloth is a CLI, not a server

The first runnable milestone is a bootable command with help and version output.
HTTP servers appear only in tests and examples. This resolves the phrase “basic
server” in favor of the PRD's explicit standalone-CLI scope.

### D-002 — Stable work identifiers

Use immutable two-digit IDs while the backlog is below 100. Branches use
`SH-XX-short-description`; issue and PR titles start with `SH-XX:`. If the
project reaches 100 items, continue with `SH-100` without renumbering old work.

### D-003 — Machine output and progress use separate streams

Successful structured output is written only to stdout. Progress, warnings, and
human diagnostics go to stderr. JSON modes never emit decoration or ANSI codes.

### D-004 — Missing measurements are typed as unavailable

An unavailable phase is represented separately from a measured zero duration
and includes a machine-readable reason when known. Renderers display `N/A`.

### D-005 — Percentiles use exact bounded samples in v1

The default and safety-limited sample counts allow an exact sorted buffer. Use
linear interpolation at index `(n-1)*p` and document it. An approximate histogram
is deferred until profiling or supported request limits justify its complexity.

### D-006 — Reports store redacted target identity

Persist scheme, host, port, path, and query keys while removing query values.
Compute compatibility fingerprints from that redacted canonical identity. Never
hash raw secrets as a substitute for redaction.

### D-007 — Overall and per-experiment aggregates are distinct

The report's overall summary cannot imply that a 100-request baseline represents
an entire 600-request suite. Each experiment has its own counts and aggregates;
the overall section explicitly covers all attempted measured traffic.

### D-008 — Warm-ups consume safety budget

Warm-up requests are excluded from latency aggregates but included in rate
limiting, total traffic accounting, errors shown as warm-up outcomes, and the
hard request budget.

### D-009 — Redirect responses are measurements by default

The transport returns the first 3xx response without following it. Explicit
opt-in follows redirects with a bounded hop count and records each hop so target
changes are visible.

### D-010 — HTTP status and transport failure are different outcomes

Completed 4xx/5xx responses are HTTP outcomes with valid timing data, not
transport failures. Success-rate policy and status distributions remain visible
to diagnostic rules.

### D-011 — Diagnostic confidence is qualitative

Confidence values describe evidence completeness and consistency, not a
probability. Thresholds, contradictions, minimum samples, and rule versions are
centralized and documented.

### D-012 — No silent report overwrite

Export uses exclusive creation semantics unless `--force` is explicit. Final
writes are atomic within the destination filesystem where supported.

### D-013 — Integrate through `dev`, release through `main`

Feature branches start from and merge into `dev`. A milestone promotion pull
request moves the tested `dev` state to `main` only after the roadmap gate passes.
Agents may commit and push verified feature work but stop before pull-request
approval or merging unless the user explicitly authorizes it.

## Proposed decisions to ratify in foundation issues

### D-014 — Exit code contract

Proposed: `0` success/no gated regression, `1` runtime or measurement failure,
`2` usage/config/report validation failure, `3` confirmed configured regression,
and `130` interruption. Cobra behavior and cross-platform constraints must be
tested before this becomes public API.

### D-015 — Safety ceilings

Defaults are already conservative, but normal ceilings and absolute ceilings
need documented values. Exceeding the normal ceiling should require an explicit
noninteractive acknowledgement flag; no flag should bypass an absolute cap.
The estimate must include every experiment and warm-up.

### D-016 — Response-body cap semantics

Proposed: stream bodies to `io.Discard` up to a centralized cap, record an
explicit truncated outcome when exceeded, and do not claim connection reuse for
a connection whose body could not be fully drained. Content-Length is metadata,
not proof of bytes transferred.

### D-017 — Compatibility severity matrix

Proposed fatal mismatches for gate mode: unreadable schema, target, method,
experiment semantics, timeout policy, concurrency stage, rate policy, and unsafe
method mismatch. Descriptive comparison may retain rows for compatible subsets
while prominently warning about noncomparable sections.

### D-018 — Connection experiment ordering

Proposed: alternate fresh and reused samples using a deterministic seeded order
recorded in the report. This reduces temporal drift without making runs
irreproducible. A transport must never be shared between the fresh and reuse
arms.

## Explicitly deferred choices

- Approximate histograms and extremely large sample sets
- A persistent configuration file
- Vendor-specific CI annotations
- Homebrew publication before release artifacts are stable
- Automatic statistical significance claims from one before/after pair
- Plugin architecture for third-party diagnostic rules
- Browser UI, hosted history, accounts, telemetry, or cloud storage

## Change protocol

When a decision changes:

1. Update this log and the context graph.
2. Identify affected issue IDs and report/schema versions.
3. Add migration or compatibility work when external behavior has shipped.
4. Do not rewrite old issue IDs or erase the original rationale.
