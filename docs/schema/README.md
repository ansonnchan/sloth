# Report schema evolution

`report-v1.schema.json` is the machine contract for `sloth.report/v1.x`.

- Major versions change when existing fields are removed, renamed, or given
  incompatible semantics.
- Minor versions may add optional fields. Readers must preserve the distinction
  between unsupported major versions and compatible newer minor versions.
- Existing fields retain their units and nullability for the entire major line.
- Every schema change updates the Go model, bundled schema copy, conformance
  tests, and committed examples in the same change.
- Arrays and strings remain explicitly bounded. New sample formats require both
  count and serialized-size ceilings.

The published copy under `docs/schema/` and the schema embedded in the binary
are byte-for-byte compared in tests.
