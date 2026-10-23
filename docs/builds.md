# Build metadata

Every Sloth binary has a typed version, commit, build time, and dirty-state
record. Local builds need no special flags and report `dev`; missing commit and
time values remain `unknown` in report metadata rather than becoming empty.

Release automation injects reproducible values through Go linker flags:

```text
-X github.com/ansonnchan/sloth/internal/buildinfo.version=<version>
-X github.com/ansonnchan/sloth/internal/buildinfo.commit=<full-commit-sha>
-X github.com/ansonnchan/sloth/internal/buildinfo.buildTime=<utc-rfc3339-time>
-X github.com/ansonnchan/sloth/internal/buildinfo.dirty=<true-or-false>
```

Release builds should also use `-trimpath` and `-buildvcs=false`; commit identity
already comes from the explicit linker value. The build time must be supplied
explicitly in UTC. Rebuilding with the same source, toolchain, flags, and
build-time value therefore produces the same metadata instead of consulting the
current checkout during compilation or tests.
