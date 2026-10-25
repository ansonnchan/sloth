# Request and target model

Sloth separates data needed to execute an HTTP request from metadata that is
safe to render or persist.

The runtime request specification owns the original URL, headers, optional body
source, timeout, redirect policy, and response-body limit. Those fields are
private and are available to the measurement engine only through defensive
copies or narrow accessors. Formatting a request specification produces only
the method and redacted target identity.

The safe target identity contains:

- a lowercase scheme and hostname;
- a non-default port, when present;
- the escaped request path;
- sorted query-key names, including repeated keys;
- a URL whose query values are replaced with `[REDACTED]`; and
- a SHA-256 fingerprint of that redacted canonical URL.

The fingerprint never hashes the original query values. Targets that differ
only in hostname case, an explicit default port, query order, or query values
therefore share an identity. A different path or query-key shape produces a
different identity.

Safe request metadata also redacts `Authorization`, `Proxy-Authorization`,
`Cookie`, `Set-Cookie`, configured sensitive header names, and header names that
identify token, secret, or API-key values. Request and response bodies are not
part of the safe metadata model.

## Request outcomes

Every attempt has one stable outcome category: `success`, `http_error`,
`timeout`, `transport_error`, `cancelled`, or `body_limit`. A completed 4xx or
5xx response is an HTTP outcome rather than a transport failure, so its status,
timings, and transferred-byte evidence remain usable.

Transport-related outcomes separately identify the client-observed failure
phase when available: DNS, connection establishment, TLS negotiation, request,
or response-body transfer. This phase is an observation, not a claim about a
server-side root cause. Partial responses keep their known status and byte count
and are explicitly marked incomplete.

Internally, classified errors retain their wrapped Go cause for cancellation and
diagnostic handling. Human and JSON forms contain only stable messages that do
not embed the original URL or transport error text.

## Raw trace evidence

Each request attempt owns an independent trace collector. It records request
start, response-header return, body completion, DNS callbacks, every connection
attempt, TLS callbacks, selected-connection reuse and idle state, request-write
callbacks, first response byte, and the protocol reported by the response.

Starts and completions remain separate ordered observations. Sloth does not
assume that a callback happens exactly once or has a matching counterpart;
multiple address attempts, overlapping work, missing hooks, and terminal errors
are valid raw records. Callback errors retain their internal causes but expose
only the same classified, secret-safe messages as request outcomes.

Trace timestamps come from `time.Now`, whose elapsed-time operations use its
monotonic reading while the process is running. Raw timestamps are inputs to
timing derivation and are not themselves the public timing report.

## Transport ownership

Each measurement arm owns its HTTP transport and connection pool. The default
and reuse profiles permit keep-alive reuse; the fresh profile disables
keep-alives so consecutive attempts do not silently share a connection. A
fresh-profile transport and a reuse-profile transport are always different
instances.

All profiles use the standard environment proxy policy, dual-stack network
dialing, bounded connection and TLS setup timeouts, system TLS verification,
and explicit HTTP/2 negotiation. The owner closes idle connections when its
measurement lifecycle ends. Transport state is never shared across experiment
arms.

## Response bodies

Response bodies are streamed directly to `io.Discard` and closed on every path;
Sloth does not retain their content. Reads stop at the configured byte limit
plus one detection byte. That byte is necessary to distinguish an exactly-full
body from a chunked or streaming body that exceeds the limit, and it is included
in the transferred-byte count.

Exceeding the limit is a typed `body_limit` outcome. Read errors, early
disconnects, truncation, and close failures leave the body incomplete and not
eligible for reuse. A completely drained and successfully closed body is only
marked reuse-eligible; actual reuse is reported separately from the next
request's trace observation. Byte counts describe the stream exposed by Go's
HTTP client, which may be decoded when automatic content decompression applies,
and never rely on `Content-Length` as proof of transferred bytes.
