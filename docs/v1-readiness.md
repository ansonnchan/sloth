# v1.0.0 release readiness review

Review date: 2026-10-29. Scope: SH-01 through SH-62. Decision: **GO for the `v1.0.0` tag after the M7 promotion pull request passes CI, platform smoke, and release packaging without changes.** This review does not itself create the tag or publish a GitHub Release.

The decision means the documented v1 behavior is demonstrated and the remaining limitations are accepted. It does not claim statistical accuracy, remote-server performance, reliability, security certification, users, or adoption.

## Product success criteria

| Criterion | Evidence | Result |
|---|---|---|
| Standalone binaries for five targets | `.goreleaser.yml`, `scripts/release-verify`, SH-60 snapshot workflow | Pass |
| Native help/version and all four workflows | `internal/releasecheck`, `.github/workflows/platform.yml`; runs on Linux AMD64/ARM64, macOS AMD64/ARM64, Windows AMD64 | Pass |
| One-request phase diagnosis | `internal/httpmeasure/*_test.go`, `internal/inspect`, CLI inspect tests | Pass |
| Bounded repeatable experiments | runner lifecycle, limiter, budget, cancellation, ceiling, benchmark, and race tests | Pass |
| Six deterministic evidence rules | Rule matrix below and `docs/diagnostics.md` | Pass |
| Privacy-safe versioned reports | privacy canaries, report schema/fixtures, sample bounds, atomic file tests | Pass |
| Compatible comparison and CI gate | comparison matrix, delta/gate tests, workflow fixtures | Pass |
| Understandable methodology and limitations | `docs/methodology.md`, `docs/timing.md`, `docs/metrics.md`, final README, real RC recording | Pass |

## Verification matrix

| Check family | Command or automation | Coverage |
|---|---|---|
| Formatting | `test -z "$(gofmt -l .)"` | All Go source |
| Static analysis | `go vet ./...` | All packages and verification commands |
| Unit/integration/end-to-end | `go test ./...` | HTTP, CLI, runner, metrics, diagnosis, reports, sample API, docs/demo |
| Concurrency | `go test -race ./...` and repeated runner race job | Worker pools, cancellation, shared state |
| Schema and fixtures | CI report-fixture step and report schema tests | Canonical report model and examples |
| Security/privacy | `privacy_test.go`, request redaction tests, threat model | Headers, cookies, URL values, bodies, errors, reports |
| Malformed input | package fuzz targets and scheduled fuzz workflow | Config, request, report, compatibility, delta arithmetic |
| Network failures | CLI/inspect local integration suites | DNS, refused/reset, TLS, timeout, partial body, redirects, proxy, cancellation |
| Platforms | Platform smoke workflow | Five native release targets, CGO disabled |
| Packaging | GoReleaser snapshot plus `scripts/release-verify` | Names, contents, checksums, metadata, installed native artifact |
| Documentation | `go run ./scripts/docs-check` | Local links and public workflow commands |

The final gate is repeated from a clean checkout. GoReleaser's host-native archive is extracted and passed through the same installed-binary harness used by platform CI; help, version, inspect, diagnose, export, comparison, exclusive creation, forced replacement, schema parsing, cleanup, and Unix permissions must all succeed.

## Six-rule evidence matrix

| Rule | Positive/negative/boundary evidence | Contradictory/sparse evidence | Public interpretation |
|---|---|---|---|
| `HIGH_FIRST_BYTE_WAIT` | `TestFirstByteRuleMatrix` | unavailable timing and confidence-policy cases | `docs/diagnostics.md#high_first_byte_wait` |
| `CONNECTION_OVERHEAD` | `TestConnectionRuleMatrix` | observed-state, protocol, and proxy limitations | `docs/diagnostics.md#connection_overhead` |
| `TAIL_VARIABILITY` | `TestTailRuleMatrix` | sample/completeness and interruption cases | `docs/diagnostics.md#tail_variability` |
| `CONCURRENCY_SENSITIVITY` | `TestConcurrencyRuleMatrix` | two-stage, achieved-load, scheduling-pressure cases | `docs/diagnostics.md#concurrency_sensitivity` |
| `ERROR_RATE` | `TestErrorRuleClassMatrix` | separated 4xx/5xx/transport/timeout/cancellation classes | `docs/diagnostics.md#error_rate` |
| `INSUFFICIENT_EVIDENCE` | `TestInsufficientEvidenceRuleMatrix` | sparse suite and strong-finding suppression cases | `docs/diagnostics.md#insufficient_evidence` |

Confidence remains evidence quality, not a probability. No rule claims a confirmed server-side cause.

## Accepted v1 limitations

- Measurements are client-observable and cannot see application, database, queue, dependency, or server-trace internals.
- HTTP/1.1 and HTTP/2 are supported through Go's standard transport; HTTP/3/QUIC is not a v1 target.
- Planned arrivals expose scheduling pressure but cannot eliminate coordinated omission or local-client confounding.
- Tail percentiles are descriptive; the default 100 samples do not justify high-confidence p99 claims.
- Proxies, VPNs, CDNs, caches, TLS resumption, multiplexing, and backend routing can alter phase availability and comparability.
- Release archives have SHA-256 checksums but are not code-signed or notarized. Checksums detect corruption, not publisher impersonation.
- Windows syncs the complete temporary file before publication but cannot request the Unix directory durability barrier through Go's standard library. Windows file permissions follow ACLs rather than POSIX mode bits.
- Homebrew, offline re-analysis, repeated-run uncertainty, phase-level gates, portable CI summaries, and richer proxy/address-family evidence remain post-v1 work (SH-63–SH-68).
- The committed demo uses a release-candidate snapshot. Final release publication replaces the RC identity only after the exact promotion head is tagged.

## Release procedure

1. Merge the M7 promotion pull request only after CI, platform smoke, and release packaging succeed.
2. Confirm `main` equals the reviewed promotion commit and has no uncommitted files.
3. Create an annotated `v1.0.0` tag at that commit and push the tag.
4. Let the least-privilege release workflow publish archives and checksums; do not upload hand-built replacements.
5. Verify the GitHub Release files against `checksums.txt`, download the host-native archive, and run `sloth --version` plus the four workflows.
6. If publication or verification fails, leave the evidence visible, do not move the tag, and publish a corrected patch version from a reviewed commit.

Candidate release notes are maintained at [v1.0.0 release notes](release-notes/v1.0.0.md).
