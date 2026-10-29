# Release artifacts and installation

Tagged `v*` commits are packaged by pinned GoReleaser `v2.18.2`. The workflow has read-only repository access for pull-request snapshots and receives `contents: write` only in the tag-publishing job.

## Artifact set

Each version produces exactly five archives plus `checksums.txt`:

```text
sloth_VERSION_darwin_amd64.tar.gz
sloth_VERSION_darwin_arm64.tar.gz
sloth_VERSION_linux_amd64.tar.gz
sloth_VERSION_linux_arm64.tar.gz
sloth_VERSION_windows_amd64.zip
checksums.txt
```

Unix archives contain `sloth`; the Windows archive contains `sloth.exe`. Every archive also includes `LICENSE`, `README.md`, and `CHANGELOG.md`. Binaries are built with `CGO_ENABLED=0`, trimmed source paths, and injected version, full commit, UTC build time, and clean-tree metadata.

## Verify and install

Download the archive for the operating system and architecture from the GitHub Release. Download `checksums.txt` into the same directory, then verify before extracting:

```sh
# Linux
sha256sum --check --ignore-missing checksums.txt

# macOS
shasum -a 256 --check checksums.txt
```

On Windows PowerShell, compare the published value with:

```powershell
(Get-FileHash .\sloth_VERSION_windows_amd64.zip -Algorithm SHA256).Hash
```

Extract the archive and place `sloth` (or `sloth.exe`) on `PATH`. Confirm the installed identity:

```sh
sloth --version
```

The output must contain the release version and full source commit. Package-manager signing and platform notarization are not part of v1; checksums provide corruption detection, not publisher identity verification.

Go users can instead compile from a tagged source revision:

```sh
go install github.com/ansonnchan/sloth/cmd/sloth@v1.0.0
```

## Reproduce a snapshot

With GoReleaser `v2.18.2` installed:

```sh
goreleaser check
goreleaser release --snapshot --clean
go run ./scripts/release-verify --dist ./dist --commit "$(git rev-parse HEAD)"
```

The verifier requires all five consistently named archives, validates every SHA-256 entry, inspects archive contents and executable names, and runs the host-native archived binary to confirm its version and commit metadata. Snapshot builds never publish a GitHub Release.
