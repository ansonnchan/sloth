# Report compatibility

Sloth checks compatibility before calculating deltas. It never converts a
missing or incompatible value into zero.

Gate mode requires exact agreement on schema semantics, redacted target
identity, method, timeout and request policies, rate, warm-ups, concurrency
stages, diagnostic rules, and each paired experiment's kind, sample count,
worker count, and rate. Any mismatch makes the evidence ineligible for a CI
decision.

Descriptive mode is intentionally more permissive. A different schema major,
target, or method prevents all comparison. Other differences produce prominent
warnings, and Sloth retains only experiment IDs present in both reports with the
same experiment kind. This permits an honest partial view without claiming that
the whole runs were equivalent.
