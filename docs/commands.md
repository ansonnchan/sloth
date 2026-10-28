# Command and flag reference

Syntax shown here matches `sloth --help`. Durations accept Go duration strings such as `250ms`, `30s`, and `2m`; byte limits and counts are integers. Global presentation flags may appear before or after a subcommand.

## Global commands and flags

```text
sloth [--color auto|always|never] [--no-color] [--ascii] COMMAND
sloth --help
sloth --version
```

| Flag | Default | Behavior and safety |
|---|---|---|
| `--color auto|always|never` | `auto` | Automatic color requires a character-device stdout. `always` never overrides `NO_COLOR` or `--no-color`. |
| `--no-color` | false | Disables ANSI output. A non-empty `NO_COLOR` environment variable has the same precedence. |
| `--ascii` | false | Replaces non-ASCII presentation characters; machine JSON is already ASCII-control safe. |
| `-h`, `--help` | false | Prints help without contacting a target. |
| `-v`, `--version` | false | Prints injected version/build information without contacting a target. |

`sloth help [COMMAND]` displays command help. `sloth completion bash|fish|powershell|zsh [--no-descriptions]` prints Cobra's shell-completion script; `--no-descriptions` defaults to false and omits command descriptions. Completion performs no network or file writes unless the user redirects it.

## `inspect`

```text
sloth inspect URL [flags]
```

Performs one bounded request. HTTP 4xx/5xx is measured evidence, not a CLI transport failure.

| Flag | Default | Behavior and safety |
|---|---|---|
| `-X`, `--method` | `GET` | Accepts GET, HEAD, OPTIONS, POST, PUT, PATCH, DELETE. State-changing methods require `--allow-unsafe-methods`. |
| `-H`, `--header` | none | Repeatable `Name: Value`. Control characters are rejected; values may reach the target but are never emitted. |
| `--body-file` | none | Opens a replayable local body only after validation. Contents are never emitted. |
| `--timeout` | `30s` | Bounds the request. Above 1 minute requires `--allow-high-load`; 5 minutes is absolute. |
| `--body-limit` | 1 MiB (`1048576`) | Bounds bytes read. Above 10 MiB requires acknowledgement; 100 MiB is absolute. |
| `--follow-redirects` | false | Enables bounded redirects; disabled requests retain the first redirect response. |
| `--redirect-limit` | `5` | Used with redirect following. Above 5 requires acknowledgement; 10 is absolute. |
| `--json` | false | Writes one `sloth.inspect/v1` document to stdout, including partial runtime evidence. |
| `--allow-unsafe-methods` | false | Explicitly acknowledges that the selected method may mutate server state. |
| `--allow-high-load` | false | Acknowledges a setting above a normal ceiling; never bypasses absolute ceilings. |

```sh
sloth inspect --json --timeout 5s https://api.example.com/health
sloth inspect -X POST --allow-unsafe-methods --body-file payload.json http://127.0.0.1:8080/test
```

## `diagnose`

```text
sloth diagnose URL [flags]
```

Runs baseline, fresh/reused connection, and concurrency experiments. Defaults produce 600 measured requests and 30 warm-ups across six arms, all sharing one five-starts/second limiter.

| Flag | Default | Behavior and safety |
|---|---|---|
| `--requests` | `100` per arm | Normal ceiling 1,000; absolute ceiling 10,000. Counts contribute to the full suite budget. |
| `--warmups` | `5` per arm | May be zero. Warm-ups consume budget/rate but never enter measured percentiles. |
| `--rate` | `5` starts/s | Aggregate, no-burst suite rate. Normal ceiling 100; absolute ceiling 1,000. |
| `--concurrency` | `1,5,10` | Increasing unique stages, each no greater than request count. Normal ceiling 50; absolute 200. |
| `--quiet` | false | Suppresses plan/progress on stderr, not warnings, errors, or results. |
| `--json` | false | Writes one aggregate `sloth.diagnose/v1` document; no raw samples. |
| `-X`, `--method` | `GET` | Same supported-method and unsafe-method policy as inspect. |
| `-H`, `--header` | none | Repeatable, validated, and excluded from output/reports. |
| `--body-file` | none | Replayed for each request after validation; contents are excluded from output. |
| `--timeout` | `30s` per request | Normal ceiling 1 minute; absolute 5 minutes. |
| `--body-limit` | 1 MiB per request | Normal ceiling 10 MiB; absolute 100 MiB. |
| `--follow-redirects` | false | Enables bounded redirects for every request. |
| `--redirect-limit` | `5` | Normal ceiling 5; absolute 10. |
| `--allow-unsafe-methods` | false | Acknowledges possible server mutation for the complete planned suite. |
| `--allow-high-load` | false | Required when any normal ceiling or the 10,000-total-request normal budget is exceeded. |

```sh
sloth diagnose --requests 30 --warmups 2 --rate 10 --concurrency 1,5 http://127.0.0.1:8080/api
sloth diagnose --json --quiet https://api.example.com/health > diagnosis.json
```

## `export`

```text
sloth export URL [flags]
```

Runs the diagnostic suite, validates `sloth.report/v1.0`, and atomically publishes a mode-0600 JSON report. It has every diagnose workload/request flag above except `--json`, with the same defaults and safety behavior, plus:

| Flag | Default | Behavior and safety |
|---|---|---|
| `-o`, `--output` | generated name in the existing Desktop | Parent directory must exist. Relative paths resolve from the working directory. |
| `-f`, `--force` | false | Allows atomic replacement of an existing report; without it, publication is exclusive. |
| `--include-samples` | false | Opts into bounded measured timing/outcome samples. Headers, bodies, raw URLs, and raw errors remain excluded. |
| `--sample-limit` | `1000` | Normal ceiling 1,000; absolute 10,000. The entire report is capped at 16 MiB. |

```sh
sloth export --quiet --output ./reports/baseline.json https://api.example.com/health
sloth export --include-samples --sample-limit 200 --force --output ./report.json http://127.0.0.1:8080/api
```

Cancellation or report validation failure leaves no final file.

## `compare`

```text
sloth compare BEFORE.json AFTER.json [flags]
```

Both reports are size-bounded, schema-validated, and checked for workload/target compatibility before deltas are calculated.

| Flag | Default | Behavior and safety |
|---|---|---|
| `--metric p50|p95|p99` | `p95` | Selects the latency percentile used by the optional CI gate. |
| `--fail-on-regression PERCENT` | disabled | Enables a nonnegative percentage threshold. Any compatible arm exceeding it returns exit 3. |

```sh
sloth compare before.json after.json
sloth compare --metric p95 --fail-on-regression 20 before.json after.json
```

Incompatible or insufficient evidence cannot silently pass a gate. See the [CLI contract](cli-contract.md) for exit codes and [report compatibility](report-compatibility.md) for exact fields.
