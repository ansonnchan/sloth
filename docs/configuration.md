# Configuration

Sloth resolves command-line values into one effective configuration before any
orchestration begins. Defaults are defined centrally in `internal/config` so
commands and reports use the same values.

## Defaults

| Setting | Default |
|---|---:|
| HTTP method | `GET` |
| Request timeout | 30 seconds |
| Measured requests per experiment | 100 |
| Warm-up requests per experiment | 5 |
| Request start rate | 5 per second |
| Concurrency stages | 1, 5, 10 |
| Follow redirects | No |
| Terminal color | Automatic |
| Response-body read limit | 1 MiB |
| Include individual samples | No |

These values describe configuration only. SH-04 defines the separate normal
and absolute safety ceilings that constrain overrides.

The serializable effective configuration contains no request URL, headers,
credentials, cookies, or request body. Durations and byte limits use explicit
`_ms` and `_bytes` field names.

## Input validation

Sloth accepts absolute `http` and `https` targets without URL user information.
Supported v1 methods are `GET`, `HEAD`, `POST`, `PUT`, `PATCH`, `DELETE`, and
`OPTIONS`; SH-04 applies the additional acknowledgement required for unsafe
methods.

Timeouts, request counts, rates, response limits, and concurrency stages must be
positive. Warm-up counts may be zero. Concurrency stages must be unique,
increasing, and no larger than the measured request count. Headers use a
`Name: Value` form and cannot contain control characters.

Validation errors identify the invalid field without repeating raw targets,
headers, credentials, or query values.
