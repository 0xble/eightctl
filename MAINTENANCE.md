# Maintenance

## Background

Maintained fork: `0xble/eightctl` of `steipete/eightctl`; maintained and
upstream-default branch `main`. Canonical checkout: `/Users/brianle/Repos/eightctl`.
Accepted upstream baseline: `db84b936e0ba107209864508b434ff0b2761b553` (fetched
2026-09-09). Publish only to `origin`; never push to `upstream`.

## Preserve

- Headless authentication uses the file-only keyring and app API calls retain
  bounded 401/429 retry behavior.
- API-drift compatibility, alarm/base payload shapes, and restored command APIs
  remain covered; source sync, publication, installation, and runtime activation
  are separate stages requiring separate authorization and proof.

## Active patches

### EIGHTCTL-001: `fix: FileBackend-only keyring + bounded retry on 401/429`

- **Status:** Active; source difference confirmed against `upstream/main` on 2026-09-09.
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

### EIGHTCTL-002: `fix: restore upstream APIs dropped during rebase conflict resolution`

- **Status:** Active; source difference confirmed against `upstream/main` on 2026-09-09.
- **Provenance:** `3fbf3eaee59dbf78faf46213942b6b46d624a6d4`; **surfaces/invariant:**
  `internal/client/{eightsleep.go,schedules.go,base.go}` and `internal/cmd/` retain
  restored API paths and payload compatibility.
- **Proof:** `go test ./internal/client ./internal/cmd`; **rollback:** revert that
  commit only after its command/API coverage remains upstream-equivalent.
- **Upstream issue/PR:** untracked; audit 2026-09-09 found none recorded. **Retire
  when:** a released upstream descendant passes this proof without these changes.

## Update

Every run fetches `origin` and `upstream`, reconciles `main` onto the latest
`upstream/main`, preserves only recorded active patches, updates this register with
any patch change/retirement, and runs focused proof plus `pnpm test && pnpm build`
before authorized publication. Missing or stale coverage blocks publication.
Immediately before `Updated` or `Already current`, fetch `upstream` again and
require zero upstream-only commits; otherwise report `Blocked` with stage, refs,
and evidence. Publish to `origin` or report that concrete blocker.

## Verify

```text
git diff --check
git rev-list --left-right --count upstream/main...main
```

Require a fresh final fetch with zero upstream-only commits and, after authorized
publication, local `main` SHA equal to `origin/main`. Installed/runtime SHA proof
is required only for separately authorized later stages.
