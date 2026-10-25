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
