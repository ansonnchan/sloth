# Real terminal demo

[`sloth.cast`](sloth.cast) is an asciinema v2 recording generated from a real GoReleaser snapshot binary, not mocked output. The recorded binary identifies itself as `0.0.1-next` at commit `2392dfb09e508c2455fad5f2fdc768b21ffcc570`.

Play it locally:

```sh
asciinema play docs/demo/sloth.cast
```

The recording starts the repository's deterministic sample API at `127.0.0.1:8080`, then runs version, inspect, and bounded diagnose workflows. Displayed latency and diagnostic metrics are genuine observations from the recording machine; they are illustrative, not product performance or accuracy claims.

## Reproduce

Build or extract a release-candidate binary, ensure port 8080 is free, then run:

```sh
go run ./scripts/demo-record --binary /path/to/sloth --output docs/demo/sloth.cast
```

The generator forces ASCII, non-color output and fails if any recorded command fails. The sample API is loopback-only and is shut down when recording completes.
