# Benchmarking client overhead

Sloth's microbenchmarks isolate client work from network and server latency. They cover request-local trace collection and phase derivation, statistical aggregation at 100 and 1,000 samples, the rendering-free worker pool at the normal 1,000-request experiment and 10,000-request suite ceilings, and JSON report serialization.

## Reproduce

Record the environment before comparing runs:

```sh
go version
go env GOOS GOARCH GOAMD64
```

Also record the CPU model and available memory using the operating system's system-information tool. Then run:

```sh
go test -run '^$' -bench . -benchmem \
  ./internal/httpmeasure ./internal/metrics ./internal/runner ./internal/report
```

For statistically stronger comparisons, install `benchstat`, save at least ten runs from each revision, and compare like-for-like machines:

```sh
go test -run '^$' -bench . -benchmem -count 10 \
  ./internal/httpmeasure ./internal/metrics ./internal/runner ./internal/report > before.txt
# build or check out the candidate revision, then repeat into after.txt
benchstat before.txt after.txt
```

Do not compare results collected under different Go versions, operating systems, architectures, power modes, or competing workloads. Benchmark numbers describe the tested environment; they are not claims about remote server performance.

## Ceiling checks

`TestSchedulerAccuracyAtNormalSuiteCeiling` advances a deterministic clock through all 10,000 request-start slots at the normal 100 requests/second limit and verifies exact no-burst spacing. `TestPoolBoundsResultsAtNormalSuiteCeiling` runs 10,000 no-I/O samples with 50 workers and verifies that results remain bounded by the planned suite size. The worker-pool benchmark's `requests-10000` case reports actual allocation growth for that ceiling with `-benchmem`.

These checks intentionally use no external endpoint and produce no terminal rendering. This keeps scheduler, runner, instrumentation, aggregation, and serialization overhead distinguishable from network variability.
