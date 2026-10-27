# Report fixture policy

The fixture suite covers aggregate-only, bounded-sample, interrupted/partial,
error-heavy, and incompatible reports. Every fixture uses fixed UTC timestamps,
stable run IDs, bounded deterministic values, and the production report model.

CI validates committed examples and generated workflow fixtures against both the
bundled JSON Schema and the typed version-aware parser. It also exercises
compatibility, delta, rendering, and gate behavior from the same fixtures.

Fixture changes are contract changes. A pull request that updates a fixture must
state the schema, compatibility, metric, or presentation rationale; incidental
golden rewrites are not accepted.
