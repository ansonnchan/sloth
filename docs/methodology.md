# Measurement methodology and limitations

Sloth runs controlled client-side observations. It can identify repeatable patterns in what one client sees; it cannot confirm an internal server, database, dependency, queue, CDN, proxy, or network root cause.

## Experimental design

The default diagnostic suite uses six sequential arms: one baseline, alternating fresh and reusable connections, then concurrency stages 1, 5, and 10. Every arm uses the same validated request configuration. Five warm-ups precede 100 measured requests per arm. Warm-ups share the suite's traffic controls but are excluded from latency distributions.

One monotonic planned-arrival schedule, no-burst start limiter, and hard request budget span the suite. Offered start rate, achieved start rate, completed throughput, scheduling delay, peak in-flight requests, and time-weighted concurrency are recorded separately. This separation prevents configured load from being mistaken for achieved load.

Fresh/reused results are interpreted only when observations confirm the intended connection behavior. Concurrency findings require a meaningful achieved-rate change; configured worker count alone is not evidence.

## Timing and statistics

The exact callback boundaries and unavailable reasons are defined in [timing semantics](timing.md). Phases are independent observations and need not sum to total latency. In particular, first-byte wait is not server execution time.

The [metric definitions](metrics.md) specify percentile interpolation, population standard deviation, rates, concurrency, and outcome reconciliation. Percentiles are descriptive: Sloth does not claim statistical significance, confidence intervals, or a population service-level objective. Diagnostic confidence describes evidence quantity/completeness and consistency, not probability that a hypothesized cause is true.

## Coordinated omission

Sloth uses planned arrivals and retains scheduling delay, which exposes some client saturation that a closed-loop "send the next request after the last finishes" test would hide. It does not synthesize latency samples for requests that were never started, and it does not extrapolate a server-side queue. When the client cannot achieve the offered schedule, the report shows missed arrivals, offered-versus-achieved rate, and scheduling delay rather than silently shifting the schedule.

This reduces but does not eliminate coordinated-omission risk. Operating-system scheduling, connection-pool blocking, local CPU pressure, proxy queues, and upstream admission control can still couple observation timing to response completion. Treat results with large scheduling delay or offered/achieved divergence as client-pressure evidence; repeat from a suitably provisioned client before attributing endpoint sensitivity.

## Known confounders and false positives

- DNS caches, TLS session resumption, connection pooling, HTTP/2 multiplexing, HTTP/3 absence, proxies, VPNs, CDNs, and load-balancer routing can change phase availability or comparability.
- Local CPU, memory, file descriptors, power mode, background traffic, and Go/runtime versions can affect scheduling and allocations.
- Rate limits, autoscaling, caches, just-in-time compilation, garbage collection, and dependency fan-out can make short runs nonstationary.
- Request method, headers, authentication, body, region, response size, and target data must stay representative; a fast health endpoint does not characterize a business endpoint.
- Small samples make tail percentiles unstable. The default 100-request arm cannot produce high-confidence p99 evidence.
- A completed HTTP 4xx/5xx response has valid latency but may describe an error path unlike successful work.
- Redirects can cross origins; followed-hop timings describe the client-visible chain, not one server.

Each diagnostic rule's specific suppression conditions and common false positives are listed in [diagnostics](diagnostics.md). Inconclusive output is intentional when the evidence cannot distinguish hypotheses.

## Reproducible practice

1. Use the same Sloth version, target identity, request configuration, client host/class, network path, and time window.
2. Record deployment and infrastructure changes outside Sloth; reports intentionally do not collect host identifiers or environment variables.
3. Run a descriptive comparison before enabling a CI threshold. Choose a threshold based on repeated baseline variation, not a single pair.
4. Inspect achieved rate, scheduling delay, outcomes, and completeness before reading latency deltas or findings.
5. Repeat surprising results and corroborate with server-side metrics/traces before changing production systems.

The local [sample API](../examples/sample-api/README.md) provides deterministic route behavior for learning and smoke tests, not a model of production performance.
