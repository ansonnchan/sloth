# Experiment runner semantics

Sloth schedules request starts from one monotonic origin. Each request retains
its original planned start even when workers are busy, and records its actual
worker start separately. The difference is scheduling delay and is not included
in client-observed HTTP service time.

An arrival dispatched at least one configured start interval late is counted as
missed. Missed arrivals are still executed unless cancellation or the request
budget stops the run; they are never silently discarded or shifted to create a
new, more favorable schedule.

One no-burst start limiter belongs to the complete diagnostic suite. Its first
request may start immediately; later starts are paced at the configured rate.
All workers, warm-ups, experiment arms, and concurrency stages share that one
limiter, so adding workers cannot multiply the configured request-start rate.
Cancelled waits count as offered starts but not achieved starts.

Each experiment stage runs through a fixed-capacity worker pool. One producer
owns and closes the job channel; workers stop through the parent context; a
collector drains results from work that had already started. A suite-wide
request budget is checked before rate admission, so budget exhaustion or
cancellation cannot launch additional requests. Partial results retain the
specific stop reason and completed evidence in request-sequence order.

Warm-ups run before measured traffic through the same executor, worker pool,
limiter, budget, and cancellation path. Their outcomes remain visible in a
separate summary, but their durations never enter measured percentiles or
throughput. HTTP-level warm-up failures do not stop the run; cancellation,
budget exhaustion, or an internal executor failure does.

The baseline experiment runs with one worker and an isolated reusable transport.
It attempts the configured measured count after warm-up unless cancellation,
budget exhaustion, or an internal runner failure stops it. HTTP errors, body
limits, and timeouts are measured outcomes rather than runner failures, so their
valid timing and response evidence remains in the baseline aggregate.

The connection experiment owns separate fresh and reused transports. Measured
samples alternate between arms using a deterministic seed recorded with the
result, while both arms share request configuration, timeout, and the suite-wide
start limiter. A comparison is sufficient only when every fresh sample observes
a new connection and the reused arm observes reuse. HTTP/2 multiplexing and
proxy behavior remain explicit caveats; configured mode alone is not evidence.

Concurrency stages always begin at one and include the requested maximum.
Default landmarks at five and ten are retained when they do not exceed that
maximum. For example, maximums of 3, 7, 10, and 15 produce `[1,3]`, `[1,5,7]`,
`[1,5,10]`, and `[1,5,10,15]`. Values above the absolute concurrency ceiling
are rejected rather than silently clamped.

Each concurrency stage has a complete one-stage configuration snapshot, an
isolated reusable transport, and its own warm-up, measured samples, and
aggregate. Stages run in ascending order but share the suite limiter and request
budget, so worker count cannot change the configured aggregate start rate.
Reports record achieved peak and time-weighted in-flight concurrency; cancelled
runs retain completed and partial stages in configured order.
