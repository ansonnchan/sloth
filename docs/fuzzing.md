# Fuzzing malformed inputs

Sloth has bounded fuzz targets for configuration and header parsing, request
construction and redaction, versioned report parsing, compatibility
classification, and delta arithmetic. Each target rejects inputs above its
documented byte cap before parsing, and the report reader retains its 16 MiB
production limit.

Run a short local campaign from the repository root:

```bash
go test ./internal/config -run=^$ -fuzz=^FuzzValidateInput$ -fuzztime=10s
go test ./internal/httpmeasure -run=^$ -fuzz=^FuzzNewRequestSpec$ -fuzztime=10s
go test ./internal/report -run=^$ -fuzz=^FuzzParseReport$ -fuzztime=10s
go test ./internal/compare -run=^$ -fuzz=^FuzzCompatibilityAndDeltas$ -fuzztime=10s
go test ./internal/compare -run=^$ -fuzz=^FuzzDeltaArithmetic$ -fuzztime=10s
```

Seed corpora include valid redacted reports, truncated JSON, malformed URLs,
URL credentials, sensitive headers, IPv6 targets, missing metrics, zero
baselines, and unavailable values. Any discovered crash or semantic regression
must be reduced to a deterministic seed before the fix is merged.

The `Scheduled fuzzing` workflow runs all targets weekly and can also be started
manually. Fuzzing is bounded by per-target time, input-size checks, and the job
timeout; it must not contact external services.
