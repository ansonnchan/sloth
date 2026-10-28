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
| HTTP 4xx rate | 0.10 | 4xx responses / non-cancelled attempts |

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

## `CONNECTION_OVERHEAD`

This rule requires recorded connection-state evidence from both alternating
fresh and reused arms. Fresh median latency must exceed reused median latency by
at least 20 ms and 20%. Evidence includes arm sizes, observed new/reused counts,
the median delta, and client-visible connect and TLS contributions. HTTP/2
multiplexing or proxy-tunnel ambiguity lowers confidence. The recommendation is
connection reuse and bounded pooling, not a claim that a particular network
layer is definitively at fault.

## `TAIL_VARIABILITY`

This rule requires baseline p95 latency to be at least twice p50 and at least
100 ms higher. Both relative and absolute thresholds avoid labeling a small
absolute spread as operationally important. Evidence includes p50, p95, p99,
their ratio and gap, sample count, and incomplete outcomes. P99 is shown only as
descriptive context; the default 100 samples cannot support a high-confidence
p99 interpretation. Sparse, invalid, interrupted, or internally inconsistent
outcomes suppress or lower confidence.

## `CONCURRENCY_SENSITIVITY`

This rule first requires at least a 25% difference in achieved request rate; a
change in configured concurrency alone is not evidence. It then evaluates p50,
p95, throughput, error counts, actual in-flight concurrency, and scheduling
delay across every stage. A 1.5x p95 increase or a one-percentage-point error
increase can identify endpoint-sensitive behavior. Scheduling delay or an
offered-versus-achieved shortfall at 20% instead produces a client-pressure
finding and lowers confidence in endpoint interpretation. Evidence lists
offered rate, achieved rate, actual concurrency, p50, p95, successes, errors,
and scheduling delay for every stage.

## `ERROR_RATE`

This rule evaluates timeouts, transport failures, HTTP 5xx responses, and HTTP
4xx responses as separate classes. Timeouts, transport failures, and 5xx cross
at 1%; non-5xx classes become high severity at 5%. Because 4xx commonly reflects
the request rather than endpoint availability, it uses a separate 10% policy
and does not become high severity by rate alone. User cancellation is reported
but excluded from the denominator and never treated as an endpoint error.

## `INSUFFICIENT_EVIDENCE`

This rule emits one structured, low-confidence inconclusive result when other
rules are blocked by sparse or unavailable measurements, contradictory inputs,
incompatible connection arms, unchanged achieved load, or interruption. Its
evidence lists concrete limitations and its recommendation names the safest
next measurement. It never lists a likely cause. The engine omits this finding
when a medium- or high-confidence positive finding already explains the
available evidence, or when all evaluated evidence is adequate and clearly
normal.

## False positives and suppressions

Rules identify client-observable patterns, not confirmed causes. Use this table before acting on a finding:

| Finding | Common confounders / false positives | Built-in guard | Required follow-up |
|---|---|---|---|
| `HIGH_FIRST_BYTE_WAIT` | Proxy/CDN buffering, cache misses, rate limiting, or network loss before first response bytes | Requires at least 100 ms and 60% of total median latency plus adequate phase completeness | Correlate with server and proxy traces; repeat from another network path |
| `CONNECTION_OVERHEAD` | DNS cache state, TLS resumption, proxy tunnels, HTTP/2 multiplexing, or different backend routing | Requires observed new/reused state plus 20 ms absolute and 20% relative delta; ambiguity lowers confidence | Repeat with controlled proxy/protocol settings and inspect connection evidence |
| `TAIL_VARIABILITY` | Small samples, nonstationary deployments, client GC/CPU pauses, mixed response sizes, or cache warm-up | Requires p95 at least 2x p50 and 100 ms higher; sparse/interrupted evidence is suppressed or lowered | Increase samples within safety limits and segment server evidence by request class |
| `CONCURRENCY_SENSITIVITY` | Client saturation, connection-pool limits, proxy queues, rate limiting, or autoscaling transitions | Requires at least 25% achieved-rate change; scheduling pressure becomes client-pressure evidence | Verify client headroom, then correlate throughput/errors with server saturation metrics |
| `ERROR_RATE` | Intentional 4xx tests, authentication expiry, canary routing, or retry intermediaries | Separates outcome classes, excludes cancellation from the denominator, and uses a 10% 4xx threshold | Inspect status/error classes and server logs; do not combine 4xx with availability failures |
| `INSUFFICIENT_EVIDENCE` | Expected unavailable phases, cancellation, incompatible arms, or unchanged achieved load | Emits no likely cause and lists the blocking limitations | Follow its named next measurement rather than inferring normality |

No rule corrects for multiple hypothesis testing or produces a causal probability. Repeated findings across comparable runs increase operational confidence but do not replace server-side corroboration. See [Measurement methodology and limitations](methodology.md).
