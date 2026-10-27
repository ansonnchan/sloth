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
aggregates and diagnostics only. Optional bounded samples are added by the
sample-export contract without changing these aggregate definitions.
