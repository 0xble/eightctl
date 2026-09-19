# Headless authentication

Part of the [root maintenance contract](../MAINTENANCE.md). Read the root's
accepted baseline and shared adoption/publication rules before this unit.
Load for every full maintenance run or changes to this responsibility.

### EIGHTCTL-001: `fix: FileBackend-only keyring + bounded retry on 401/429`

- **Provenance:** `159f3223ff15437c3dc2dd0ea2f3657819f9ad60`; follow-up test
  `8fa268aff47c1279426f93bd0d71f9349f3cceff`.
- **Surfaces/invariant:** `internal/tokencache/{tokencache.go,tokencache_test.go}`
  and `internal/client/{eightsleep.go,eightsleep_test.go}` keep headless auth
  noninteractive and retries finite.
- **Proof:** `go test ./internal/tokencache ./internal/client`; **rollback:**
  revert the provenance commit and its regression test together.
- **Upstream issue/PR:** untracked; the 2026-09-09 audit records no association,
  not an absence claim. Live-check and type as direct/associated/related before change.
- **Retire when:** released upstream supplies the complete invariant and the same
  focused proof passes after removing the fork delta.

## Update and verification

Compare the candidate upstream implementation with each retained behavior above.
Keep its provenance and adoption/retirement decision with this unit when it changes.
Run the focused proof named above and the root verification gate before claiming
maintenance success.
