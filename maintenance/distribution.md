# Fork distribution

Part of the [root maintenance contract](../MAINTENANCE.md). Read the root's
accepted baseline and shared adoption/publication rules before this unit.
Load for every full maintenance run or changes to this responsibility.

### EIGHTCTL-003: `fix(fork): retain fork install and version identity`

- **Status:** Active. The upstream base is v0.2.8 and the established fork
  suffix is preserved as `0.2.8-0xble.0.1.0` in Go and package metadata.
- **Provenance:** `d3a42879f4032eb138b44d04595354eed5209f19`,
  `746ac4766c2b8f661a8b116e0f9b62684cc34c6f`, and
  `aa942141be851a78c13dbb4f8ee87a5c48797f2d`.
- **Surfaces/invariant:** `bin/{upgrade,smoke}`, `internal/cmd/version.go`, and
  `package.json` keep the fork-specific upgrade/smoke and version identity coherent
  without performing installation.
- **Proof:** `sh -n bin/upgrade bin/smoke && go build ./cmd/eightctl`; **rollback:** revert this
  family together only after equivalent build, version, and upgrade/smoke coverage.
  **Upstream disposition:** upstream release tooling does not replace the fork's
  `bin/upgrade`, `bin/smoke`, or fork identity. **Fork delivery:** maintained
  `origin/main`. **Retire when:** a separately authorized runtime migration removes
  the fork distribution flow.

## Update and verification

Compare the candidate upstream implementation with each retained behavior above.
Keep its provenance and adoption/retirement decision with this unit when it changes.
Run the focused proof named above and the root verification gate before claiming
maintenance success.
