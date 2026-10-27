# Reading saved reports

Sloth reads at most 16 MiB from a report file. Missing, unreadable, oversized,
malformed, truncated, unsupported-major, and schema-validation failures retain
distinct machine-readable error kinds and actionable paths.

Readers accept `sloth.report/v1.x`. A newer minor version is marked as such,
known fields are validated against the v1 contract, and unknown top-level fields
are retained as raw JSON for forward-compatible workflows. Any other major
version is rejected before comparison. A file must contain exactly one JSON
value; trailing values or garbage are invalid.

Typed decoding preserves JSON `null` duration values as unavailable pointers.
It never substitutes numeric zero for missing evidence. Parser fuzz coverage
uses the same 16 MiB bound and must not panic.
