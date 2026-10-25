# Experiment runner semantics

Sloth schedules request starts from one monotonic origin. Each request retains
its original planned start even when workers are busy, and records its actual
worker start separately. The difference is scheduling delay and is not included
in client-observed HTTP service time.

An arrival dispatched at least one configured start interval late is counted as
missed. Missed arrivals are still executed unless cancellation or the request
budget stops the run; they are never silently discarded or shifted to create a
new, more favorable schedule.
