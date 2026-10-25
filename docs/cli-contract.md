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
- `--follow-redirects` with `--redirect-limit`; and
- the `--allow-unsafe-methods` and `--allow-high-load` acknowledgements.

Validation and safety policy run before Sloth opens a body file or contacts the
target. Ctrl+C cancels the in-flight request and exits with status 130. HTTP
4xx/5xx responses remain successful measurements; timeouts, transport errors,
body-limit failures, and cancellation use their documented nonzero statuses.
