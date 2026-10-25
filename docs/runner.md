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
