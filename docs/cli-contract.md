# CLI output and exit contract

Sloth keeps machine-readable data separate from human diagnostics:

- Successful structured output is written only to stdout.
- Progress, warnings, and errors are written only to stderr.
- JSON output is undecorated and contains no ANSI control sequences.
- Quiet mode suppresses progress, but not warnings, errors, or final data.
- Runtime failures print one error and do not print command usage.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | Success, including no configured regression |
| `1` | Runtime or measurement failure |
| `2` | Command usage, configuration, or report validation failure |
| `3` | A configured comparison regression was confirmed |
| `130` | The run was interrupted |

These values are public automation behavior. New error categories must map to
one of these codes unless a separately reviewed contract change is required.

## Inspect command

`sloth inspect URL` performs one bounded request. It supports:

- `--method` / `-X` for GET, HEAD, POST, PUT, PATCH, or DELETE;
- repeatable `--header` / `-H` values in `Name: Value` form;
- `--body-file` for a replayable request body;
- `--timeout` and `--body-limit` bounds;
- `--follow-redirects` with `--redirect-limit`;
- `--json` for machine-readable output;
- `--color auto|always|never` and `--ascii` presentation controls; and
- the `--allow-unsafe-methods` and `--allow-high-load` acknowledgements.

Validation and safety policy run before Sloth opens a body file or contacts the
target. Ctrl+C cancels the in-flight request and exits with status 130. HTTP
4xx/5xx responses remain successful measurements; timeouts, transport errors,
body-limit failures, and cancellation use their documented nonzero statuses.

Human output includes the method, redacted endpoint, status, negotiated
protocol, observed connection reuse, response bytes, total duration, each
timing phase, and exactly one bounded interpretation. Missing or inapplicable
phases render as `N/A` with a reason. The first-byte interval is described as a
client-observed wait that can include server work, network, queueing, and
upstream delay—not as server execution time.

`inspect --json` writes one ANSI-free `sloth.inspect/v1` document to stdout.
Once measurement has begun, runtime and interruption outcomes still produce a
document before the process returns its nonzero exit status; validation failures
do not. See [Inspect JSON](inspect-json.md) for the field and nullability rules.

## Diagnose command

`sloth diagnose URL` runs the standard baseline, alternating fresh/reused
connection, and configured concurrency experiments through the six-rule
diagnostic engine. Defaults are 100 measured requests and five warm-ups per arm,
five aggregate request starts per second, and concurrency stages 1, 5, and 10.
The default suite therefore contains 600 measured requests plus 30 warm-ups.

Before opening a body file or contacting the target, Sloth validates all flags,
enforces the safety policy, and writes the effective configuration, planned
request count, and minimum schedule time to stderr. The suite shares one rate
limiter and one hard request budget. `--quiet` suppresses plan and progress
lines. Ctrl+C stops scheduling, retains and diagnoses partial evidence, prints
the partial summary, and exits with status 130.

Human output contains aggregate tables for baseline, connection, and concurrency
arms; each row includes sample count, p50, p95, p99, errors, offered and achieved
rates, and actual time-weighted concurrency. Findings keep observations,
possible causes, recommendations, confidence, and typed evidence visibly
separate, followed by limitations and next steps. Noninteractive output is
static and uses no cursor-control animation.

`diagnose --json` writes one ANSI-free `sloth.diagnose/v1` aggregate document to
stdout. Plan and progress lines remain on stderr, or are suppressed with
`--quiet`; they never contaminate the JSON document. The command does not expose
request or response bodies, headers, or raw samples. See
[Diagnose JSON](diagnose-json.md) for the transient command-output contract; the
persisted report schema is introduced separately by the export milestone.

## Export command

`sloth export URL` runs the standard diagnostic suite, validates a canonical
`sloth.report/v1.0` document, and atomically writes it to the existing Desktop
or `--output` path. Existing targets require `--force`. Plan and progress stay
on stderr; successful stdout contains only the absolute final path.

`--include-samples` opts into timing/outcome samples, bounded by
`--sample-limit` (1,000 by default, 10,000 maximum) and the 16 MiB report limit.
Cancellation or validation failure leaves no final file.

## Compare command

`sloth compare BEFORE.json AFTER.json` validates both files, checks workload
compatibility, and renders descriptive before/after deltas. Compatibility
warnings always precede metrics and conclusions.

`--fail-on-regression PERCENT` enables the CI gate. Its default metric is p95;
`--metric p50|p95|p99` selects another latency percentile. The gate uses raw,
unrounded percentage changes for every compatible experiment. Malformed,
incompatible, missing, zero-baseline, or interrupted evidence cannot pass by
default: validation uses exit 2, confirmed regression exit 3, and an interrupted
saved run exit 130.
