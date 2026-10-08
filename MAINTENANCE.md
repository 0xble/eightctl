# Maintenance

## Background

`0xble/eightsleep` began as `0xble/eightctl`, a maintained fork of
`steipete/eightctl`, and was rewritten on github.com/0xble/toolkit on
2026-10-07 as a Tool Fleet tool. The rewrite replaces the fork's command layer,
so upstream commits no longer merge. The `upstream` remote stays for reference.
Publish only to `origin`; never push to `upstream`.

## Preserve

- Headless authentication uses the file-only keyring and app API calls retain
  bounded 401/429 retry behavior.
- Every command, flag, output and exit code recorded in
  [docs/compatibility.md](docs/compatibility.md) stays, unless a change is
  listed there; `internal/compat` holds the proof.

## Maintenance units

| Unit | Purpose / required behavior | Load when | Contract |
| --- | --- | --- | --- |
| Headless authentication | File-only token storage and bounded retries | Every full run or changes to this responsibility | [Headless authentication](maintenance/headless-auth.md) |
| API compatibility | Retained command APIs and provider payload shapes | Every full run or changes to this responsibility | [API compatibility](maintenance/api-compatibility.md) |

## Update

Review upstream's default branch for provider fixes (endpoint moves, payload
changes) and port each by hand into `internal/client` with a test. Do not merge
upstream. Run `./bin/ci gate` before publication.

## Verify

`./bin/check` and `./bin/ci gate <sha>` pass on the published head, and the
exact-head `qualification` check is green.
