# Fork CI

Part of the [root maintenance contract](../MAINTENANCE.md). Load on full maintenance runs and CI changes.

### EIGHTCTL-004: exact-SHA fork qualification

- **Required behavior:** `bin/ci` owns portable preflight, full PR gate, and nightly checks; `gate.yml` validates the PR head and aggregates core plus both Go compatibility versions as `qualification`. Nightly validates `github.sha`, repeats both compatibility lanes, and adds an uncached shuffled race suite. Setup precedes the script's exact-SHA and clean-tracked-tree assertions. Release fixtures must not inherit the expected SHA.
- **Decision:** Upstream `ci.yml` at `26a7c1daf5d3be284b35f22848f90cf959d471cd` differs from the fork's `origin/main` release-artifact version assertion (`dist/metadata.json` versus fork `package.json`). Because it is already fork-modified, replace it with one PR gate rather than maintain two competing PR workflows. Leave upstream-owned `dependency-graph.yml` unchanged: its path-filtered push/dispatch submission is not a PR gate. Keep dispatch-only release workflows untouched.
- **Contribution route:** Fork-only CI policy; upstream PR #95 supplies the pinned-action and Go compatibility precedent, not this fork qualification design. No upstream contribution planned. **Fork delivery:** PR to `0xble/eightctl` (merge pending). **Proof:** `./bin/ci preflight`, `./bin/ci gate HEAD_SHA`, negative SHA and dirty-tree probes, workflow actionlint, and an exact-head GitHub `qualification` result. **Retire when:** equivalent upstream qualification covers the fork-specific release metadata and desired exact-SHA policy; restore upstream-owned CI only after proving no coverage loss.

## Update and verification

On upstream sync, map each new CI check into the profiles before adopting changes. Verify the gate on its exact fork head and account for both compatibility jobs; keep the path-filtered dependency snapshot independent.
