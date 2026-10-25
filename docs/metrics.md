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
