# Report files

When no output path is supplied, Sloth writes a name such as
`sloth-20261027T213045Z-a1b2c3d4.json` to the current user's existing Desktop.
The timestamp is UTC and the random suffix prevents collisions between runs in
the same second. Sloth does not create a missing Desktop; use `--output` on
headless systems.

Explicit relative paths are resolved against the working directory. Their
parent directory must already exist.

Reports are written to a mode-`0600` temporary file in the destination
directory, synced, closed, and then published. Publication is exclusive by
default, so an existing path produces an error. `--force` is required to replace
an existing report. Failed writes clean up their temporary file and do not leave
a partial final report.
