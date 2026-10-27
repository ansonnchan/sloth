# Diagnose JSON

`sloth diagnose --json URL` writes exactly one `sloth.diagnose/v1` document to
stdout. The document is the aggregate command result, not the persisted report
format used by the later export workflow.

Top-level fields are:

- `schema_version`: always `sloth.diagnose/v1`;
- `rule_version`: the deterministic diagnostic ruleset;
- `configuration`: the effective, secret-free configuration snapshot;
- `plan`: arm, measured, warm-up, total-request, and minimum-duration values;
- `experiments`: ordered aggregate summaries for completed or partial arms;
- `findings`: ordered findings with observations, possible causes,
  recommendations, qualitative confidence, severity, and typed evidence; and
- `limitations`: ordered qualifications that apply to interpreting the run.

Each experiment summary contains sample count, p50/p95/p99 in milliseconds,
error count, offered and achieved starts per second, time-weighted actual
concurrency, and interruption state. Unavailable latency values are JSON `null`,
not zero.

Progress, the preflight traffic plan, warnings, and errors remain on stderr.
JSON output contains no ANSI sequences, request or response bodies, request
headers, raw samples, or URL query values. A run interrupted after measurement
begins still writes its partial document before exiting with status 130.
