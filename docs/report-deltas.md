# Report delta semantics

Sloth calculates deltas only after compatibility classification. Absolute
change is always `after - before`. Percentage change uses the unrounded values
and is `(after - before) / before * 100`; it is unavailable when the baseline is
zero or either value is unavailable.

Every metric carries direction metadata. Lower is better for latency,
scheduling delay, errors, timeouts, and new-connection overhead. Higher is
better for throughput, success, and connection reuse. In-flight concurrency is
neutral evidence. Presentation may round values, but regression gates always
use the raw values.

The v1 delta set includes mean and p50/p90/p95/p99 latency, scheduling delay,
completed throughput, error/timeout counts and rates, success rate, connection
counts and reuse rate, concurrency, and available p50/p95 phase timing.
