# M6 client-overhead benchmark evidence

This evidence is a reproducibility record, not a cross-machine performance promise.

- Revision lineage: SH-56 merged to `dev` as `49cf188927a8d0431fe3541b568ed3a737842fb0`.
- Collected: 2026-10-28.
- Go: `go1.27.1`.
- OS/architecture: `darwin/arm64`.
- CPU reported by Go benchmark output: Apple M2, 8 logical CPUs (`-8`).
- Command: `go test -run '^$' -bench . -benchmem -benchtime=100x -count=5 ./internal/httpmeasure ./internal/metrics ./internal/runner ./internal/report`.

The table reports the median of five fixed 100-iteration benchmark samples. Allocation values are the median reported after encoder cache warm-up where applicable.

| Benchmark | Median ns/op | Median B/op | Median allocs/op |
|---|---:|---:|---:|
| Trace collector | 4,667 | 2,392 | 68 |
| Derive timing phases | 1,880 | 936 | 34 |
| Aggregate 100 samples | 41,948 | 11,992 | 114 |
| Aggregate 1,000 samples | 214,169 | 103,064 | 1,014 |
| Render-free pool, 1,000 requests | 989,370 | 1,640,225 | 100 |
| Render-free pool, 10,000 requests | 8,981,770 | 22,882,274 | 116 |
| Serialize canonical report | 11,778 | 4,244 | 7 |

The pool benchmarks use deterministic no-I/O executors and include bounded scheduling/result storage; they do not measure network or server latency. Fixed iteration counts make the record easy to reproduce but less statistically strong than time-calibrated, randomized multi-run comparisons with `benchstat`. Use the procedure in [Benchmarking client overhead](../benchmarking.md) for change decisions.
