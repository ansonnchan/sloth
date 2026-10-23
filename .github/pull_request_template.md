## Linked issue

Closes #

## Summary

- Summarize the focused change.

## Verification

- [ ] `test -z "$(gofmt -l .)"`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] `go test -race ./...`
- [ ] `go build ./...`
- [ ] Relevant cross-build targets pass

## Documentation

- [ ] User-visible commands, flags, defaults, timing semantics, limits, diagnostics, or schemas are documented
- [ ] No documentation change is required

## Safety and privacy

Describe effects on traffic bounds, unsafe methods, redirects, cancellation,
URLs, headers, bodies, persisted data, and machine-readable output.

- [ ] The change preserves conservative traffic and resource bounds
- [ ] Tests prove secrets and bodies are not exposed by default
- [ ] No safety or privacy behavior is affected

## Checklist

- [ ] The branch is named `SH-XX-short-description` and targets `dev` (except a dedicated milestone promotion)
- [ ] The PR title starts with the same `SH-XX` ID
- [ ] The change is limited to one independently verifiable outcome
- [ ] Newly discovered scope is recorded as a follow-up issue
