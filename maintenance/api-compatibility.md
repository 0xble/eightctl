# API compatibility

Part of the [root maintenance contract](../MAINTENANCE.md). Read the root's
accepted baseline and shared adoption/publication rules before this unit.
Load for every full maintenance run or changes to this responsibility.

### EIGHTCTL-002: `fix: restore upstream APIs dropped during rebase conflict resolution`

- **Status:** Adapted to upstream's split client/transport architecture at
  `26a7c1daf5d3be284b35f22848f90cf959d471cd`. The old fork-local `doApp`
  consolidation is superseded by upstream PR #92; retained compatibility now
  lives in the split endpoint owners and focused tests.
- **Provenance:** `3fbf3eaee59dbf78faf46213942b6b46d624a6d4`; **surfaces/invariant:**
  `internal/client/{eightsleep.go,schedules.go,base.go}` and `internal/cmd/` retain
  restored API paths and payload compatibility.
- **Proof:** `go test ./internal/client ./internal/cmd`; **rollback:** revert that
  commit only after its command/API coverage remains upstream-equivalent.
- **Upstream disposition:** Upstream PRs #24, #31, #36, #37, #92, #97, #98,
  and #108 supersede portions of the original broad drift patch. Fork-only base
  angle and alarm payload compatibility, endpoint fallback, and recorded command
  surfaces remain intentional divergences. **Fork delivery:** maintained
  `origin/main`. **Retire when:** a released upstream descendant passes this proof
  without the retained differences.

## Update and verification

Compare the candidate upstream implementation with each retained behavior above.
Keep its provenance and adoption/retirement decision with this unit when it changes.
Run the focused proof named above and the root verification gate before claiming
maintenance success.
