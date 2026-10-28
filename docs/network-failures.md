# Network failure behavior

Sloth integration tests reproduce failures using loopback listeners, local TLS
servers, deterministic dialers, and a local proxy. They do not depend on public
DNS or third-party endpoints.

| Scenario | Stable outcome | Expected evidence |
|---|---|---|
| DNS lookup failure | `transport_error`, DNS phase | partial timings, redacted target |
| Refused connection | `transport_error`, connect phase | no response status |
| TLS verification failure | `transport_error`, TLS phase | no response body |
| Connection reset | `transport_error` | retained scheduling/attempt evidence |
| Request timeout | `timeout` | bounded by per-request timeout |
| Truncated body | `transport_error`, response-body phase | status and transferred bytes retained |
| Redirect loop/limit | `transport_error` | bounded, redacted redirect hops |
| Configured proxy | normal HTTP outcome | proxy sees the intended absolute request target |

`inspect --json` writes partial structured evidence before returning exit 1 for
these runtime failures. Query values and credentials do not appear in either
stream. Cancellation returns exit 130; the local integration bound is one
second, deliberately looser than the expected scheduler response so loaded CI
hosts do not create false failures.

The complete network suite runs under the race detector as part of the milestone
gate. Failures represented as HTTP outcomes (including 4xx/5xx) remain completed
measurements rather than transport errors.
