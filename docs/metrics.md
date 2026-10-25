# Metric definitions

## Latency statistics

Sloth retains the complete measured duration list within its configured sample
bound and sorts a copy for aggregation. Warm-up durations are not inputs.

Minimum, maximum, arithmetic mean, median, p90, p95, p99, and population
standard deviation are measured in nanoseconds internally. A percentile at
probability `p` uses linear interpolation at sorted index `(n-1)*p`. Empty sets
are unavailable rather than zero-filled. A singleton has zero population
deviation and all percentiles equal its one value. Negative durations are
invalid.

## Experiment aggregates

Outcome counts cover every started measured request and keep completed HTTP
errors separate from transport errors, timeouts, cancellation, and body-limit
outcomes. Status classes, protocols, bytes, and observed new/reused connections
come from request evidence rather than requested configuration.

Offered rate, achieved start rate, and completed-request throughput divide their
respective counts by the complete experiment wall duration. Peak concurrency is
the maximum number of overlapping actual request intervals. Time-weighted
in-flight concurrency integrates those intervals over the same complete wall
duration; it is not the configured worker count. Scheduling delay has its own
latency distribution and is never added to HTTP service duration.

Empty and interrupted experiments remain valid aggregates and carry explicit
limitations. Missing phase observations reduce only that phase's sample count.
