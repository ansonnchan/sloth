# Supported platforms

Sloth v1 publishes CGO-free binaries for five targets:

| Target | Archive binary | Native CI smoke |
|---|---|---|
| Linux AMD64 | `sloth` | `ubuntu-24.04` |
| Linux ARM64 | `sloth` | `ubuntu-24.04-arm` |
| macOS AMD64 | `sloth` | `macos-15-intel` |
| macOS ARM64 | `sloth` | `macos-15` |
| Windows AMD64 | `sloth.exe` | `windows-2025` |

Every pull request to `dev` or `main` cross-builds all five targets with `CGO_ENABLED=0`. The platform workflow also builds and executes each target on its native GitHub-hosted runner. The portable smoke harness verifies `--help`, injected version/commit metadata, JSON inspect against an ephemeral loopback endpoint, exclusive report creation, `--force` replacement, schema parsing, cleanup, and Unix report permissions.

## Platform behavior and limits

- Linux and macOS report files are created with mode `0600`. Windows permissions follow the destination directory's ACL; POSIX mode bits are not a Windows security boundary.
- Export writes and syncs a temporary file in the destination directory before publishing it. A same-filesystem hard link provides exclusive publication. Forced replacement uses the operating system's rename primitive.
- Linux and macOS sync the destination directory after publication. The Go standard library does not provide Windows directory-handle syncing, so Windows syncs the completed file before atomic publication but cannot request the additional directory durability barrier.
- `Ctrl+C` maps to `os.Interrupt` on all supported targets. Unix termination signals outside that portable interrupt contract are not promised on Windows.
- Terminal auto-detection and path separators are native. JSON schemas, exit codes, stdout/stderr separation, and ASCII mode remain platform-independent.

The sample API and CI endpoints bind only to an ephemeral loopback port. Native smoke results validate behavior on the named runner images; cross-build jobs separately ensure every release target compiles from a clean checkout.
