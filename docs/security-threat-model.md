# Security and privacy threat model

## Trust boundaries

Sloth accepts targets, headers, request bodies, proxy configuration, and report
paths from the local operator. Raw values exist only at the request boundary and
are intentionally sent to the selected endpoint (and configured proxy). They
must not cross into terminal rendering, structured command output, saved
reports, samples, debug formatting, fixtures, or stable errors.

The persisted identity retains scheme, normalized host and port, escaped path,
query-key names, a URL whose query values are `[REDACTED]`, and a fingerprint of
that redacted identity. Authorization, proxy authorization, cookie, set-cookie,
and configured secret headers are replaced by `[REDACTED]`. Bodies are streamed
and never modeled in result or report types. Cross-origin redirects strip
sensitive headers; redirect targets and transport errors use redacted identities
and stable classifications.

Report files are created with mode `0600` where the platform honors Unix-style
permissions. Optional samples contain only timing, outcome, status, byte,
protocol, and observed connection evidence.

## Residual risks

- The destination server, configured proxy, operating system, debugger, crash
  dumper, and another process with sufficient privileges may observe runtime
  secrets or process memory. Sloth does not provide memory isolation.
- Hostnames, paths, query-key names, timing, status, response sizes, and bounded
  environment metadata are intentionally retained and may themselves be
  sensitive. Operators should inspect reports before sharing them.
- Same-origin redirects preserve request credentials because they remain within
  the operator-selected origin. A compromised origin can observe them.
- A user-selected output directory or later permission change can weaken report
  confidentiality. Atomicity protects completeness, not access after creation.
- Stable redaction prevents accidental disclosure; it is not anonymization and
  does not guarantee that retained metadata cannot identify a service.

Privacy regression tests seed canaries into URL credentials, query values,
authorization/cookie/custom headers, bodies, transport failures, report samples,
and Go debug formatting. The test must prove that intended local requests receive
the canaries while every supported output boundary excludes them.
