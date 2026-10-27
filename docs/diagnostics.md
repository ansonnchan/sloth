# Diagnostic methodology

Sloth's diagnostic engine evaluates completed, bounded experiments. Rules do
not perform network, filesystem, clock, or rendering work. Their inputs are the
recorded experiment results and one versioned threshold profile.

## Finding contract

Every finding has a stable code, rule version, severity, qualitative
confidence, title, observation, possible causes, recommendation, and typed
evidence. An observation states only what Sloth measured. Possible causes are
hypotheses; they are never presented as a confirmed server-side root cause.
Numeric evidence always carries a unit and ordered evidence keeps repeated runs
deterministic.

## Version 1 threshold profile

All values live in `internal/diagnosis.DefaultThresholds`. Durations below use
elapsed time.

| Threshold | Value | Unit |
| --- | ---: | --- |
| Minimum samples | 10 | samples per evaluated data set |
| Medium-confidence samples | 30 | samples |
| High-confidence samples | 100 | samples |
| High-confidence p99 samples | 200 | samples |
| Minimum completeness | 0.50 | available / attempted ratio |
| Medium completeness | 0.80 | available / attempted ratio |
| High completeness | 0.95 | available / attempted ratio |
| First-byte share | 0.60 | first-byte wait / total latency ratio |
| Minimum first-byte wait | 100 | milliseconds |
| Connection delta | 0.20 | (fresh - reused) / fresh ratio |
| Minimum connection delta | 20 | milliseconds |
| Tail ratio | 2.0 | p95 / p50 ratio |
| Minimum tail gap | 100 | milliseconds |
| Minimum achieved-load change | 0.25 | relative rate change |
| Concurrency p95 increase | 1.50 | later-stage / reference ratio |
| Scheduling-delay share | 0.20 | scheduling delay / total latency ratio |
| Warning error rate | 0.01 | errors / attempted ratio |
| High error rate | 0.05 | errors / attempted ratio |

Changing a value or its interpretation requires a diagnostic rule-version
change and updated boundary tests. Confidence is an evidence-quality label, not
a probability or statistical significance claim.

## Evidence strength

A rule first checks that the planned sample count is known, available counts are
valid, at least 10 usable samples exist, and at least half of the planned
evidence is available. Evidence below either minimum is suppressed and can feed
the `INSUFFICIENT_EVIDENCE` finding.

Adequate evidence is low confidence by default. It becomes medium at 30 samples
and 80% completeness, or high at 100 samples and 95% completeness. Inconsistent,
contradictory, or interrupted evidence is retained only at low confidence with
an explicit limitation. Any finding that depends on p99 needs at least 200
samples for high confidence, so the default 100-request arm cannot make a
high-confidence p99 claim.

## `HIGH_FIRST_BYTE_WAIT`

This rule compares baseline median first-byte wait with median total latency. It
requires first-byte wait to be at least 100 ms and at least 60% of total latency.
Its observation is limited to the client-visible interval before response
headers. Server processing, dependency latency, server queueing, and otherwise
unisolated network delay are listed only as possible causes. Missing, sparse,
interrupted, or internally contradictory phase evidence suppresses or lowers
the finding according to the evidence-strength policy.
