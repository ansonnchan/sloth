# HTTP timing semantics

Sloth reports client-observable elapsed intervals. It does not infer time spent
inside an application server, database, queue, or upstream service.

## Boundaries

| Phase | Start | End |
|---|---|---|
| Total | Immediately before `RoundTrip` | Body completion or terminal read/request error |
| DNS | Each `DNSStart` | Its corresponding `DNSDone` |
| Connect | Each `ConnectStart` | Its corresponding `ConnectDone` |
| TLS | Each `TLSHandshakeStart` | Its corresponding `TLSHandshakeDone` |
| Request write | `WroteHeaders` | `WroteRequest` |
| First-byte wait | `WroteRequest` | `GotFirstResponseByte` |
| Body transfer | Last `GotFirstResponseByte` | Body completion or terminal read error |

Repeated DNS, connection, TLS, request-write, and first-byte intervals are
matched in timestamp order. Their union is reported, so overlapping attempts
are not double-counted. An unmatched or negatively ordered event set is marked
unavailable rather than guessed.

Connection setup is inapplicable when the selected connection was reused. TLS
is also inapplicable for a cleartext HTTP request. Missing, incomplete,
out-of-order, reused, and cleartext values carry distinct machine-readable
reasons and render as `N/A`; they are never converted to zero.

An observed zero-duration interval remains a valid measurement. It is distinct
from an unavailable interval in both the domain model and JSON representation.

## Why phases may not sum to total

These intervals are independent observations, not guaranteed pieces of one
arithmetic decomposition. DNS and connection attempts may overlap, proxies may
introduce different setup work, reused connections omit earlier phases, and
protocol implementations may not emit every hook. The total request duration is
therefore authoritative for the whole client-observed operation, while phases
explain only the intervals Sloth actually observed.

In particular, first-byte wait includes every client-observable delay after the
request write and before response bytes arrive. It must not be described as pure
server execution time.
