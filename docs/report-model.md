# Canonical report model

`sloth.report/v1.0` is the persisted evidence contract used by export and
comparison. It is distinct from the transient `sloth.diagnose/v1` command
output.

The top level keeps schema, tool, and diagnostic-rule versions separate. Run
metadata contains a collision-resistant run ID, UTC creation and execution
timestamps, and interruption state. Target identity is the already-redacted
scheme, host, normalized port, path, query-key list, redacted URL, and SHA-256
fingerprint computed from that redacted canonical identity.

Environment metadata is deliberately bounded to operating system, architecture,
Go runtime version, and logical CPU count. Reports never model hostnames,
usernames, environment variables, interfaces, raw URLs, headers, request bodies,
or response bodies.

## Aggregation boundaries

`overall.measured_outcomes` and `overall.response_bytes` reconcile all measured
experiment attempts. `overall.warmup_attempts` and `overall.warmup_outcomes` are
separate, so warm-up timing can never enter measured latency statistics. Latency,
scheduling, phase, rate, concurrency, protocol, and connection aggregates remain
per experiment.

Every serialized duration field names its unit (`_ms`). Unavailable statistics
use JSON `null`; a measured zero remains numeric zero. The default report contains
aggregates and diagnostics only.

## Optional samples

`sloth export --include-samples` adds at most `--sample-limit` measured samples;
the default limit is 1,000 and the hard maximum is 10,000. `sample_export`
records how many samples were available and included and whether the array was
truncated. The complete serialized report is also capped at 16 MiB.

Samples contain experiment identity, sequence, UTC scheduling timestamps,
millisecond phase durations, outcome classification, observed connection state,
protocol, status, and byte count. They cannot represent URLs, headers, bodies,
or underlying error messages. Warm-up samples are never exported.
