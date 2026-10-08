# Compatibility Inventory

The observable contract of `eightctl` and of the `eightsleepctl` script before
the toolkit rewrite, and where each part lives in `eightsleep` afterwards.

- `eightctl` was recorded on 2026-10-07 from `origin/main` at `a2b8291`
  (version `0.2.8-0xble.0.1.0`), built with `GOWORK=off`, by reading every file
  under `cmd/` and `internal/`, running `--help` on every command and probing
  the error paths offline with a sandboxed `HOME`.
- `eightsleepctl` was recorded from `~/dotfiles/dot_local/bin/executable_eightsleepctl`
  (identical to the installed `~/.local/bin/eightsleepctl`).

`internal/compat` replays the invocations below against an `httptest` fake of
the three Eight Sleep hosts and compares the result with golden output recorded
from both old programs (see [Caller Compatibility Test](#caller-compatibility-test)).

## Names

| Old | New |
| --- | --- |
| module `github.com/steipete/eightctl` | `github.com/0xble/eightsleep` |
| binary `eightctl` | `eightsleep`, plus `eightctl` as an alias of the same binary (a second GoReleaser build of `./cmd/eightsleep`, and `aliases: [eightctl]` in the fleet manifest) |
| script `eightsleepctl` | absorbed: every command maps to an `eightsleep` operation ([below](#eightsleepctl)). The script is retired in dotfiles after release |

Help and usage text name the program `eightsleep` under either binary name.

## Root Flags

Every root flag is accepted before or after the command, as cobra allowed.

| Flag | Default | Env fallback | Config key | Old use | New |
| --- | --- | --- | --- | --- | --- |
| `--config` | `~/.config/eightctl/config.yaml` | `EIGHTCTL_CONFIG` | | config file | root flag. Default prefers `~/.config/eightsleep/config.yaml` when it exists, else the old path |
| `-v, --verbose` | false | `EIGHTCTL_VERBOSE` | `verbose` | debug log on stderr | unchanged |
| `--email` | | `EIGHTCTL_EMAIL` | `email` | account | unchanged |
| `--password` | | `EIGHTCTL_PASSWORD` | `password` | account | unchanged, never printed |
| `--client-id` | public app client | `EIGHTCTL_CLIENT_ID` | `client_id` | OAuth client | unchanged |
| `--client-secret` | public app client | `EIGHTCTL_CLIENT_SECRET` | `client_secret` | OAuth client | unchanged, never printed |
| `--user-id` | | `EIGHTCTL_USER_ID` | `user_id` | user override | unchanged |
| `--timezone` | `local` | `EIGHTCTL_TIMEZONE` | `timezone` | API `tz`, default dates | root flag bound to input `timezone` |
| `--output` | `table` | `EIGHTCTL_OUTPUT` | `output` | `table`, `json` or `csv` for row commands; anything else prints a table | unchanged for human output, read by the render hooks |
| `--fields` | none | `EIGHTCTL_FIELDS` | `fields` | filter row keys and set column order in every `--output` format | toolkit builtin: filters `--json`/`--agent` output only (C4) |
| `--quiet` | false | `EIGHTCTL_CONFIG_QUIET` | | suppress `Using config file: ...` | unchanged |
| `--version` | | | | prints the version, exit 0 | toolkit builtin, same output |
| `-h, --help` | | | | help, exit 0 | toolkit builtin |
| `--json`, `--agent`, `-y/--yes` | | | | did not exist | toolkit builtins (C3) |

Every env variable above also accepts the `EIGHTSLEEP_` prefix, which wins over
`EIGHTCTL_`. Flags win over env, env over the config file, as in viper.

Settings precedence for credentials: flag, env, config file. The config file is
YAML. `EIGHTCTL_QUIET` or `quiet: true` in the config suppresses the `away on`
and `away off` confirmation line.

## Config And Token Cache

| Path | Old | New |
| --- | --- | --- |
| `~/.config/eightctl/config.yaml` | default config. `Using config file: <path>` on stderr unless `--quiet`. A group- or world-readable file logs a permissions warning | still read, so no re-login is needed. `~/.config/eightsleep/config.yaml` wins when it exists |
| `~/.config/eightctl/keyring/` | 99designs file keyring, service `eightctl`, key `oauth-token_v2_<base64(identity)>`, identity = API base, client ID, email | unchanged, so cached tokens keep working. Reads no longer create the directory, so a preview in a fresh `HOME` changes nothing |
| `~/.config/eightctl/token-cache.json` | legacy plaintext cache, only removed by `logout` | unchanged |
| `~/.config/eightctl/daemon.pid` | `daemon` pid file | unchanged |
| `~/.config/eightsleepctl/token.json` | the script's own plaintext token cache | not read. `eightsleep` uses the keyring cache above |

The password, client secret and tokens are never printed or written to any
output, error or log line.

## Exit Codes

| Code | Old `eightctl` | Old `eightsleepctl` | New |
| --- | --- | --- | --- |
| 0 | success, help, `--version`, bare invocation (prints help) | success | success, help, `--version` |
| 1 | every failure, including unknown commands and flags | `SystemExit` messages, HTTP errors (traceback) | unclassified and provider errors |
| 2 | | argparse usage errors | usage: parse errors, bare invocation, flag validation, `confirmation_required` |
| 3 | | | `not_found`: provider 404 |
| 5 | | | `auth_required` (no credentials or cached token), `auth_failed` (token request refused, 401 after retries), `forbidden` (403) |
| 6 | | | `rate_limited`: 429 after retries |
| 7 | | | `timeout` |

Old failures printed cobra's `Error: <message>` plus the command usage, then a
timestamped `FATAL <message>` log line, on stderr, and nothing on stdout. New
failures print `error: <message>` (or the JSON envelope under `--json` and
`--agent`) on stderr, and nothing on stdout (C1, C2).

## Commands

`C` is the client API (`client-api.8slp.net/v1`, retried once on
`app-api.8slp.net/v1` when the route is missing), `A` the app API
(`app-api.8slp.net`), `T` the token endpoint (`auth-api.8slp.net/v1/tokens`).
Every command needs credentials or a cached token unless noted. `U` is the
resolved user ID: `--user-id`, then the cached token's user, then `C GET /users/me`.
`row` means the old `printRows` output: a table, CSV or a JSON array of the
listed keys per `--output`. Writes printed nothing on success unless a line is
given.

| Command | Flags (default) | API | Old output | New |
| --- | --- | --- | --- | --- |
| `on` | `--side`, `--target-user-id` | `A PUT /v1/users/{id}/temperature` per target | `pod turned on[ for ...]` | op `on`, write, immediate |
| `off` | same | same | `pod turned off[ for ...]` | op `off`, write, immediate |
| `temp <value>` | same. Value `68F`, `20C` or a level `-100..100`, negative values allowed without `--` | `A PUT .../temperature` twice per target (smart, then level) | `temperature set (level N)[ for ...]` | op `temp`, write, immediate |
| `status` | `--side`, `--target-user-id`, `--all-sides` | household discovery, `A GET .../temperature` per side | row `side,name,user_id,mode,level`, or `mode,level` without a household | op `status` |
| `tracks` | | `C GET /users/U/audio/tracks` | row `id,title,type` | op `tracks` |
| `feats` | | `C GET /release/features` | row `title,body` | op `feats` |
| `whoami` | | none when `--user-id` or a cached token gives the ID, else `C GET /users/me` | `UserID: <id>` (any `--output`) | op `whoami`. `--json` adds `token_expires_at` from the cache |
| `logout` | | none, clears the token cache | `Logged out (token cache cleared)` | hand-written CLI command |
| `version` | | none, no credentials | version line | op `version`, not an MCP tool |
| `presence` | `--from`, `--to` (`YYYY-MM-DD`, default yesterday to today in the timezone) | `C GET /users/U/trends?...` | row `present` | op `presence` (default command, also `presence check`) |
| `daemon` | `--dry-run`, `--sync-state`, `--pid-file` | temperature writes on the config `schedule:` | log lines, runs until signalled | hand-written CLI command |
| `alarm list` | | `C GET /users/U/alarms` | row `id,time,enabled,days,vibration,sound` | op `alarm.list` |
| `alarm create` | `--time*`, `--days*` (ints), `--disabled`, `--no-vibration`, `--sound` | `C POST /users/U/alarms` | `created alarm <id>` | op `alarm.create`, write, immediate |
| `alarm update <id>` | `--time`, `--days`, `--enabled` (sent only when given), `--no-vibration` (sent only when given), `--sound` | `C PATCH /users/U/alarms/<id>` | `updated` | op `alarm.update`, write, immediate |
| `alarm delete <id>` | | `C DELETE /users/U/alarms/<id>` | `deleted` | op `alarm.delete`, destructive, immediate, the command line confirms |
| `alarm snooze <id>` | | `C POST /users/U/alarms/<id>/snooze` | | op `alarm.snooze`, write, immediate |
| `alarm dismiss <id>` | | `C POST /users/U/alarms/<id>/dismiss` | | op `alarm.dismiss`, write, immediate |
| `alarm dismiss-all` | | `C POST /users/U/alarms/active/dismiss-all` | | op `alarm.dismiss_all`, write, immediate (C11) |
| `alarm vibration-test` | | `C POST /users/U/vibration-test` | | op `alarm.vibration_test`, write, immediate |
| `away on` | `--both`, `--side`, `--target-user-id` | `A PUT /v1/users/{id}/away-mode` per target | `away mode activated (<scope>)` | op `away.on`, write, immediate |
| `away off` | same | same | `away mode deactivated (<scope>)` | op `away.off`, write, immediate |
| `away status` | same | `A GET /v1/users/{id}/away-mode` per side | row `side,name,user_id,away` | op `away.status` |
| `schedule list` | | `A GET /v1/users/U/temperature` | row `smart`, or `no Autopilot schedule configured for this user` | op `schedule.list` |
| `sleep day` | `--date` (today in the timezone) | `C GET /users/U/trends` | row `date,score,duration,latency_asleep,latency_out,tnt,resp_rate,heart_rate,hrv_score` | op `sleep.day` |
| `sleep range` | `--from*`, `--to*` | trends once per day | row `date,score,duration,tnt,resp_rate,heart_rate,hrv_score` | op `sleep.range` |
| `tempmode nap on/off/extend` | | `C POST /users/U/temperature/nap-mode/{activate,deactivate,extend}` | | ops `tempmode.nap.on/off/extend`, write, immediate |
| `tempmode nap status` | | `C GET .../nap-mode/status`, idle falls back to `.../nap-mode` | row of the payload's sorted keys | op `tempmode.nap.status` |
| `tempmode hotflash on/off` | | `C POST .../hot-flash-mode/{activate,deactivate}` | | ops `tempmode.hotflash.on/off`, write, immediate |
| `tempmode hotflash status` | | `C GET .../hot-flash-mode` | row of sorted keys | op `tempmode.hotflash.status` |
| `tempmode events` | `--from`, `--to` (dates widened to RFC 3339 day bounds) | `C GET /users/U/temp-events` | row `events` | op `tempmode.events` |
| `audio tracks` | | `C GET /users/U/audio/tracks` | row `id,title,type` | op `audio.tracks` |
| `audio categories` | | `C GET /audio/categories` | row `data` | op `audio.categories` |
| `audio state` | | `C GET /users/U/audio/player` | row `state` | op `audio.state` |
| `audio play` | `--track` | `C POST /users/U/audio/player` | | op `audio.play`, write, immediate |
| `audio pause` | | same | | op `audio.pause`, write, immediate |
| `audio seek` | `--position` (0) | `C POST .../player/seek` | | op `audio.seek`, write, immediate |
| `audio volume` | `--level` (50) | `C POST .../player/volume` | | op `audio.volume`, write, immediate |
| `audio pair` | | `C POST /devices/D/audio/player/pair` | | op `audio.pair`, write, immediate |
| `audio next` | | `C GET .../recommended-next-track` | row `next` | op `audio.next` |
| `audio favorites list` | | `C GET .../favorites` | row `favorites` | op `audio.favorites.list` |
| `audio favorites add/remove` | `--track*` | `C POST`/`DELETE .../favorites` | | ops `audio.favorites.add/remove`, write, immediate |
| `base info`, `base presets` | | `C GET /users/U/base[/presets]` | row `info`, `presets` | ops `base.info`, `base.presets` |
| `base angle` | `--head` (0), `--foot` (0) | `C POST /users/U/base/angle` | | op `base.angle`, write, immediate |
| `base preset-run` | `--name` | `C POST /users/U/base/presets` | | op `base.preset_run`, write, immediate |
| `base test` | | `C POST /devices/D/vibration-test` | | op `base.test`, write, immediate |
| `device info/peripherals/owner/warranty/online/priming-tasks/priming-schedule` | | `C GET /devices/D[/...]` | row named after the command | ops `device.*` |
| `metrics trends` | `--from`, `--to` | `C GET /users/U/trends` | row `trends` | op `metrics.trends` |
| `metrics intervals` | `--id` | `C GET /users/U/intervals/<id>` | row `interval` | op `metrics.intervals` |
| `metrics summary/aggregate/insights` | | `C GET /users/U/metrics/{summary,aggregate}`, `/insights` | row named after the command | ops `metrics.*` |
| `autopilot details/history/recap` | | `C GET /users/U/autopilot...` | row named after the command | ops `autopilot.*` |
| `autopilot level-suggestions`, `snore-mitigation` | `--enabled` (true) | `C PUT ...` | | ops, write, immediate |
| `travel trips/plans/tasks/airport-search/flight-status` | `--trip`, `--plan`, `--query`, `--flight` | `C GET ...` | row `trips`, `plans`, `tasks`, `airports`, `flight` | ops `travel.*` |
| `travel create-trip` | `--destination`, `--start-date`, `--end-date`, `--trip-timezone` | `C POST /users/U/travel/trips` | | op `travel.create_trip`, write, immediate |
| `travel delete-trip` | `--trip*` | `C DELETE /users/U/travel/trips/<id>` | | op `travel.delete_trip`, destructive, immediate, the command line confirms |
| `travel create-plan` | `--trip*`, `--name`, `--date` | `C POST .../trips/<id>/plans` | | op `travel.create_plan`, write, immediate |
| `travel update-plan` | `--plan*`, `--name`, `--date` | `C PATCH .../plans/<id>` | | op `travel.update_plan`, write, immediate |
| `household summary/schedule/current-set/invitations/devices/users/guests` | | `A GET /v1/household/users/U/...` | row named after the command | ops `household.*` |

`*` marks values the command requires. `D` is the current device: the user's
`currentDevice.id`, else its first device.

## Household Targeting

1. `on`, `off` and `temp` with neither `--side` nor `--target-user-id` act on
   every discovered household user, and on the authenticated user when
   discovery fails or finds nobody. A malformed household user response is an
   error, never a silent fallback.
2. `--side left|right|solo` must match exactly one discovered user.
   `--target-user-id` is used as given, labelled from discovery when it can be.
   Both together are refused.
3. `away on`/`off` default to the authenticated user's side. `--both` writes
   every household user and conflicts with `--side` and `--target-user-id`.
4. `away status` and `status --all-sides` read every discovered side.

## Safety Gates

| Gate | Old | New |
| --- | --- | --- |
| Immediate writes | every write applied on the command | unchanged on the CLI (`CLIImmediate`). `--dry-run` previews without writing. HTTP and MCP apply only with `"apply": true`, and `serve` refuses applied writes by default |
| Deletes | `alarm delete` and `travel delete-trip` applied with no prompt | unchanged on the CLI (`CLIConfirmed`). They are `destructive`, so HTTP and MCP also need `"confirm": true` |
| Credentials | flags, env or config. A cached token suffices | unchanged. Logout and the daemon stay CLI-only |
| Token refresh | a 401 clears the cache and re-authenticates, up to 3 retries. 429 backs off 2, 4, 6 s | unchanged |

## eightsleepctl

The script read the same config file (`--config`, default
`~/.config/eightctl/config.yaml`) and the same `EIGHTCTL_*` variables.
`--output text` printed one compact JSON line, `--output json` indented JSON
with sorted keys. Exit 1 on errors, 2 on argparse usage errors.

| Script command | API | Output | `eightsleep` equivalent |
| --- | --- | --- | --- |
| `whoami` | `T` (or its cache) | `{user_id, token_expires_at}` | `eightsleep whoami --json`: `{user_id, token_expires_at}`, the expiry present when a cached token exists |
| `alarm list` | `A GET /v1/users/U/alarms` | `{alarms: [raw alarm]}` | `eightsleep alarm list --json`: `[{id,time,enabled,days,vibration,sound}]`, one row per alarm, from the client API |
| `alarm active` | `A GET /v1/users/U/alarms/active` | `{alarms: [raw alarm]}` | `eightsleep alarm active --json`: same shape (new op `alarm.active`) |
| `alarm dismiss-all` | `A PUT /v1/users/U/alarms/active/dismiss-all`, on 404 or 405 `alarm active` then `POST .../alarms/<id>/dismiss` each | `{ok, method[, dismissed_alarm_ids]}` | `eightsleep alarm dismiss-all --json`: same shape and requests |
| `alarm dismiss-all --dry-run` | `GET /v2/users/U/routines` | `{dry_run, primary, fallback, next_alarm_id}` | `eightsleep alarm dismiss-all --dry-run --json`: same shape |
| `presence` | `C GET /users/U/current-device`, `C GET /devices/D?filter=...`, `C GET /users/U/trends` | `{present, reason, last_signal?, age_seconds?, present_window_seconds?, absent_window_seconds?, device?}` | `eightsleep presence detail --json`: same shape and heuristic (new op `presence.detail`). `EIGHTSLEEPCTL_PRESENCE_MAX_AGE_SECONDS` and `EIGHTSLEEPCTL_ABSENCE_MIN_AGE_SECONDS` still set the windows, as do `--present-within` and `--absent-after` |

The script required `client_id` and `client_secret` in the config. `eightsleep`
defaults them to the public app client, as `eightctl` did.

## Callers

Searched on 2026-10-07 for `eightctl`, `eightsleepctl`, `eightsleep` and
`Eight Sleep` (case-insensitive):

| Location | Result |
| --- | --- |
| `~/Repos/scripts/src` | none |
| `~/.hermes/cron/jobs.json` (read-only) | none. Cron output logs mention `eightctl` only as a fork maintenance target |
| `~/Repos/agents/sources` (all skills, including `health`) | none |
| `~/Repos/brianle` (apps, TypeScript) | none |
| `~/Repos/lpg` | none |
| `~/dotfiles` | only the `eightsleepctl` script itself. Not in `tools/manifest.yaml` |
| `~/Library/LaunchAgents`, `crontab` | none (no `eightctl daemon` service) |

No caller parses `--help`, `Usage:`, version strings, error text or exit codes.

## Caller Compatibility Test

`internal/compat` holds one case per command shape above, recorded from the old
`eightctl` and from the old `eightsleepctl` against the same fake, plus the
error paths. See the PR for the result.

## Intentional Changes

See the PR body for the final list. Planned:

| # | Change | Why | Affected callers |
| --- | --- | --- | --- |
| C1 | Exit codes follow the family table (usage 2, not found 3, auth 5, rate 6, timeout 7) instead of 1 for every failure. A bare invocation exits 2 instead of printing help with 0 | family contract, which the fleet contracts check | none |
| C2 | Failure stderr is one `error: <message>` line (or the JSON envelope) instead of cobra usage plus a `FATAL` log line | toolkit CLI | none |
| C3 | New `--json` and `--agent`: the operation's result. For row commands it is the array `--output json` printed | toolkit builtin | additive |
| C4 | `--fields` filters `--json`/`--agent` output only; it no longer filters or reorders `--output table/csv/json`, and the `fields` config key and env are ignored | toolkit builtin. The render hook cannot see it | none |
| C5 | Writes accept `--dry-run` (preview, no change) and a hidden `--apply` | `CLIImmediate` | additive |
| C6 | Env and config defaults for command flags (for example `EIGHTCTL_TIME` for `alarm create --time`), an accident of viper's global binding, are ignored. Root settings keep them | flags belong to the command | none |
| C7 | New operations `alarm active` and `presence detail`, and `serve`, `mcp`, `metadata` | absorbing `eightsleepctl`, fleet design | additive |
| C8 | Help and usage are kong's | toolkit CLI | none |
| C11 | `alarm dismiss-all` uses the route `eightsleepctl` verified (2026-02-12: `PUT` on the app API, which advertises `Allow: PUT`), falling back to dismissing each active alarm. The old `POST` on the client API is gone | the script's verified route; a `POST` to a route that advertises only `PUT` fails, though this was not re-checked live | none |
