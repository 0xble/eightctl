# Headless authentication

Part of the [root maintenance contract](../MAINTENANCE.md). Read the root's
accepted baseline and shared adoption/publication rules before this unit.
Load for every full maintenance run or changes to this responsibility.

### EIGHTCTL-001: file-only keyring with upstream transport retries

- **Status:** Adapted. File-only storage remains a fork divergence. The bounded
  retry implementation now follows upstream PRs
  [#92](https://github.com/steipete/eightctl/pull/92),
  [#94](https://github.com/steipete/eightctl/pull/94), and
  [#97](https://github.com/steipete/eightctl/pull/97) at accepted upstream
  revision `26a7c1daf5d3be284b35f22848f90cf959d471cd`.
- **Provenance:** Original file-only change
  `159f3223ff15437c3dc2dd0ea2f3657819f9ad60`; follow-up test
  `8fa268aff47c1279426f93bd0d71f9349f3cceff`. Fork delivery is the maintained
  `origin/main` synchronization containing this record.
- **Surfaces/invariant:** `internal/tokencache/{tokencache.go,tokencache_test.go}`
  and `internal/client/{eightsleep.go,eightsleep_test.go}` keep headless auth
  noninteractive and retries finite.
- **Proof:** `go test ./internal/tokencache ./internal/client`; **rollback:**
  revert the provenance commit and its regression test together.
- **Upstream disposition:** Retry mechanics are adopted from released upstream;
  upstream still permits platform keychains, so the file-only divergence remains.
- **Retire when:** released upstream supplies the complete invariant and the same
  focused proof passes after removing the fork delta.

## Update and verification

Compare the candidate upstream implementation with each retained behavior above.
Keep its provenance and adoption/retirement decision with this unit when it changes.
Run the focused proof named above and the root verification gate before claiming
maintenance success.
