# Contributing to Sloth

Sloth changes are organized as small, independently verifiable `SH-XX` issues.
Read [AGENTS.md](AGENTS.md) and the linked planning documents before changing
product behavior.

## Issue and branch workflow

1. Use the next assigned issue ID; never reuse or renumber an existing ID.
2. Start from the latest `dev` branch.
3. Create a lowercase branch named `SH-XX-short-description`.
4. Keep the change limited to the issue's acceptance criteria. Record newly
   discovered scope as a follow-up issue.
5. Open a pull request titled `SH-XX: Concise title` against `dev`.
6. Promote `dev` to `main` only through a milestone release pull request after
   its roadmap gate passes.

Example:

```bash
git switch dev
git pull --ff-only origin dev
git switch -c SH-09-model-http-requests
```

## Required checks

Run the same commands enforced by CI before committing:

```bash
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
go build ./...
```

CI also verifies the supported build targets:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false ./cmd/sloth
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -buildvcs=false ./cmd/sloth
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false ./cmd/sloth
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -buildvcs=false ./cmd/sloth
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -buildvcs=false ./cmd/sloth
```

Use `gofmt -w` on changed Go files before running the formatting check. Add unit
tests for business logic and local `httptest` integration tests for HTTP
behavior. Avoid assertions that rely on tight wall-clock timing.

## Commit and pull-request expectations

- Use one short imperative commit subject, beginning with the issue ID.
- Do not add commit bodies, trailers, or `Co-authored-by` lines.
- Explain verification evidence in the pull request.
- Update documentation when behavior, defaults, limits, timing semantics,
  diagnostics, output, or schemas change.
- State the safety and privacy impact, including whether URLs, headers, bodies,
  request volume, redirects, filesystem output, or machine output are affected.
- Never commit secrets, generated reports, or unsupported benchmark claims.

The canonical roadmap and issue contracts live in
[`docs/planning`](docs/planning). Keep templates short and link there instead of
duplicating design rationale.
