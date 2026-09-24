# Maintenance

## Background

Maintained fork: `0xble/eightctl` of `steipete/eightctl`; the maintained branch is
`main`. The named upstream branch means upstream's live default branch, resolved on
every run before fetching; it is not statically pinned to `main`. Canonical checkout:
`/Users/brianle/Repos/eightctl`.
Accepted upstream baseline: `26a7c1daf5d3be284b35f22848f90cf959d471cd`
(fetched 2026-09-22). Publish only to `origin`; never push to `upstream`.

## Preserve

- Headless authentication uses the file-only keyring and app API calls retain
  bounded 401/429 retry behavior.
- API-drift compatibility, alarm/base payload shapes, and restored command APIs
  remain covered; source sync, publication, installation, and runtime activation
  are separate stages requiring separate authorization and proof.

## Maintenance units

The root is the sole enrollment and scheduling unit. A full maintenance run
accounts for every row, including no-change reviews. Scoped changes load their
unit and named dependencies before acting.

| Unit | Purpose / required behavior | Load when | Contract |
| --- | --- | --- | --- |
| Headless authentication | File-only token storage and bounded retries | Every full run or changes to this responsibility | [Headless authentication](maintenance/headless-auth.md) |
| API compatibility | Retained command APIs and provider payload shapes | Every full run or changes to this responsibility | [API compatibility](maintenance/api-compatibility.md) |
| Fork distribution | Coherent fork upgrade, smoke, and version identity | Every full run or changes to this responsibility | [Fork distribution](maintenance/distribution.md) |
| Fork CI | Exact-SHA qualification without duplicate PR workflows | Every full run or CI changes | [Fork CI](maintenance/fork-ci.md) |

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
