# Agent operating notes for `eightsleep`

## Writes change a real bed

`temp`, `on`, `off`, `away`, alarm, audio and base commands act on the pod the
moment they run on the command line. Preview with `--dry-run` when unsure. Over
MCP and HTTP a write needs `"apply": true`; deletes also need `"confirm": true`.

## Know whose side you are changing

`on`, `off` and `temp` without `--side` or `--target-user-id` act on every
household member, not only the account holder. `away on/off` default to the
account holder's side; `--both` covers everyone.

## Presence is an estimate

`presence` and `presence detail` infer occupancy from recent biometric samples.
`presence detail` reports `present: null` when the newest sample is neither
fresh nor stale; say so rather than guessing.

## Layout

Every command except `logout` and `daemon` is an operation in `ops/`, declared
once on toolkit; `cmd/eightsleep/main.go` only calls `toolkit.Main`. The
provider client stays in `internal/client`, the token cache in
`internal/tokencache` and the schedule runner in `internal/daemon`. Keep a
command's spelling, flags, output and exit code unless the change is listed in
`docs/compatibility.md`; `internal/compat` replays every command shape against
goldens from the old `eightctl` and `eightsleepctl` and fails on a difference.
Tests use `internal/eightfake` and never reach Eight Sleep.
