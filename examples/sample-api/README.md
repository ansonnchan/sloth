# Sloth sample API

This intentionally small server provides reproducible local behavior for learning and end-to-end tests. It is not hardened or supported as production infrastructure.

Start it from the repository root:

```sh
go run ./examples/sample-api
```

The command binds to an ephemeral loopback port and prints the full URL. Stop it with `Ctrl-C`; it completes a graceful HTTP shutdown. A different loopback port can be selected with `--listen 127.0.0.1:8080`. Non-loopback addresses are rejected.

Routes:

- `/fast` responds immediately.
- `/delay?ms=100` delays response headers by a bounded duration.
- `/stream?chunks=5&interval_ms=50` flushes bounded response chunks over time.
- `/error?status=503` returns a selectable 4xx or 5xx response.
- `/concurrency?delay_ms=25` increases latency with observed in-flight requests and reports the count in `X-Sloth-In-Flight`.
- `/redirect?to=/fast` returns a local redirect.
- `/healthz` returns a simple health response.

If the printed URL is `http://127.0.0.1:54321`, example Sloth workflows are:

```sh
go run ./cmd/sloth inspect http://127.0.0.1:54321/fast
go run ./cmd/sloth diagnose http://127.0.0.1:54321/concurrency?delay_ms=25
go run ./cmd/sloth export --output before.json http://127.0.0.1:54321/delay?ms=100
go run ./cmd/sloth export --output after.json http://127.0.0.1:54321/delay?ms=100
go run ./cmd/sloth compare before.json after.json
```

Comparison requires the same target identity in both reports, so keep the route and query identical between the two exports.
