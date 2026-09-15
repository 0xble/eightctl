# Maintenance

## Background

Maintained fork: `0xble/eightctl` of `steipete/eightctl`; the maintained branch is
`main`. The named upstream branch means upstream's live default branch, resolved on
every run before fetching; it is not statically pinned to `main`. Canonical checkout:
`/Users/brianle/Repos/eightctl`.
Accepted upstream baseline: `db84b936e0ba107209864508b434ff0b2761b553` (fetched
2026-09-09). Publish only to `origin`; never push to `upstream`.

## Preserve

- Headless authentication uses the file-only keyring and app API calls retain
  bounded 401/429 retry behavior.
- API-drift compatibility, alarm/base payload shapes, and restored command APIs
  remain covered; source sync, publication, installation, and runtime activation
  are separate stages requiring separate authorization and proof.

## Active patches

All entries are active; source differences were confirmed against upstream's
then-current default branch on 2026-09-09.

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

### EIGHTCTL-002: `fix: restore upstream APIs dropped during rebase conflict resolution`

- **Provenance:** `3fbf3eaee59dbf78faf46213942b6b46d624a6d4`; **surfaces/invariant:**
  `internal/client/{eightsleep.go,schedules.go,base.go}` and `internal/cmd/` retain
  restored API paths and payload compatibility.
- **Proof:** `go test ./internal/client ./internal/cmd`; **rollback:** revert that
  commit only after its command/API coverage remains upstream-equivalent.
- **Upstream issue/PR:** untracked; audit 2026-09-09 found none recorded. **Retire
  when:** a released upstream descendant passes this proof without these changes.

### EIGHTCTL-003: `fix(fork): retain fork install and version identity`

- **Provenance:** `d3a42879f4032eb138b44d04595354eed5209f19`,
  `746ac4766c2b8f661a8b116e0f9b62684cc34c6f`, and
  `aa942141be851a78c13dbb4f8ee87a5c48797f2d`.
- **Surfaces/invariant:** `bin/{upgrade,smoke}`, `internal/cmd/version.go`, and
  `package.json` keep the fork-specific upgrade/smoke and version identity coherent
  without performing installation.
- **Proof:** `sh -n bin/upgrade bin/smoke && go build ./cmd/eightctl`; **rollback:** revert this
  family together only after equivalent build, version, and upgrade/smoke coverage.
  **Upstream issue/PR:** untracked; audit 2026-09-09 records no association, not an
  absence claim. **Retire when:** a separately authorized runtime migration removes
  the fork distribution flow.

## Update

Every run resolves upstream's live default branch before fetching it, then fetches
`origin` and `upstream` separately, reconciles `main` onto the latest
`upstream/$UPSTREAM_DEFAULT`, preserves only recorded active patches, updates this
register with any patch change/retirement, and runs focused proof plus
`go test ./... && go build ./cmd/eightctl` before authorized publication. Missing or
stale coverage blocks publication. Immediately before `Updated` or `Already current`,
resolve and fetch upstream's live default branch again and require zero upstream-only
commits; otherwise report `Blocked` with stage, refs, and evidence. Publish to
`origin` or report that concrete blocker.

## Verify

Require a fresh final fetch with zero upstream-only commits and, after authorized
publication, local `main` SHA equal to `origin/main`. Installed/runtime SHA proof
is required only for separately authorized later stages.
