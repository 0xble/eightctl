# API compatibility

Part of the [root maintenance contract](../MAINTENANCE.md). Read the root's
accepted baseline and shared adoption/publication rules before this unit.
Load for every full maintenance run or changes to this responsibility.

### EIGHTCTL-002: `fix: restore upstream APIs dropped during rebase conflict resolution`

- **Provenance:** `3fbf3eaee59dbf78faf46213942b6b46d624a6d4`; **surfaces/invariant:**
  `internal/client/{eightsleep.go,schedules.go,base.go}` and `internal/cmd/` retain
  restored API paths and payload compatibility.
- **Proof:** `go test ./internal/client ./internal/cmd`; **rollback:** revert that
  commit only after its command/API coverage remains upstream-equivalent.
- **Upstream issue/PR:** untracked; audit 2026-09-09 found none recorded. **Retire
  when:** a released upstream descendant passes this proof without these changes.

## Update and verification

Compare the candidate upstream implementation with each retained behavior above.
Keep its provenance and adoption/retirement decision with this unit when it changes.
Run the focused proof named above and the root verification gate before claiming
maintenance success.
