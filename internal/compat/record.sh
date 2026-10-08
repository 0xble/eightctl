#!/usr/bin/env bash
# Re-record the caller-compatibility goldens from the pre-toolkit programs.
# Usage: internal/compat/record.sh [git-ref] [eightsleepctl-script]
# The default ref is a2b8291, origin/main immediately before the rewrite. The
# default script is the dotfiles source of ~/.local/bin/eightsleepctl at the
# pinned commit below, read from dotfiles' git history rather than the working
# tree. Any script, default or given, must match the pinned sha256.
# Each old program is copied to a temporary directory and changed in exactly
# one way: its Eight Sleep hosts read EIGHTSLEEP_COMPAT_BASE, so it reaches
# the test fake instead of the real service.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

# The eightsleepctl the goldens were recorded from.
script_repo="$HOME/dotfiles"
script_path="dot_local/bin/executable_eightsleepctl"
script_commit="57e47d09ed5b7382b598ac1ef2ba39bb2905ea62"
script_sha256="78ba73a13146b0f55886602a784eb553a1077566ea70a22a8c9415e8f4185e2b"

ref="${1:-a2b8291}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/src"

if [ -n "${2:-}" ]; then
  script="$2"
  cp "$script" "$tmp/eightsleepctl-src"
else
  script="$script_repo@$script_commit:$script_path"
  git -C "$script_repo" show "$script_commit:$script_path" >"$tmp/eightsleepctl-src"
fi
got="$(shasum -a 256 "$tmp/eightsleepctl-src" | cut -d' ' -f1)"
if [ "$got" != "$script_sha256" ]; then
  printf 'error: %s has sha256 %s, want %s (dotfiles %s)\n' "$script" "$got" "$script_sha256" "$script_commit" >&2
  exit 1
fi

# eightsleepctl uses PEP 604 unions (X | None), which need Python 3.10.
python=""
for candidate in python3.14 python3.13 python3.12 python3.11 python3.10 python3; do
  if command -v "$candidate" >/dev/null 2>&1 &&
    "$candidate" -c 'import sys; sys.exit(sys.version_info < (3, 10))' 2>/dev/null; then
    python="$(command -v "$candidate")"
    break
  fi
done
if [ -z "$python" ]; then
  echo "error: eightsleepctl needs Python 3.10 or later; none found on PATH" >&2
  exit 1
fi

git archive "$ref" | tar -x -C "$tmp/src"

client="$tmp/src/internal/client/eightsleep.go"
cat >"$tmp/src/internal/client/compat_host.go" <<'GO'
package client

import "os"

// compatHost returns the test fake's prefix for an Eight Sleep host.
func compatHost(name string) string {
	if base := os.Getenv("EIGHTSLEEP_COMPAT_BASE"); base != "" {
		return base + "/" + name
	}
	return "https://" + name + ".8slp.net"
}
GO
perl -0pi -e 's/\nconst \(\n\tdefaultBaseURL/\nvar (\n\tdefaultBaseURL/' "$client"
perl -pi -e 's#"https://(client-api|app-api|auth-api)\.8slp\.net#compatHost("$1") + "#g' "$client"
grep -q 'compatHost("auth-api")' "$client"
grep -q 'compatHost("client-api")' "$client"
(cd "$tmp/src" && GOWORK=off go build -o "$tmp/eightctl-old" ./cmd/eightctl)

cp "$tmp/eightsleepctl-src" "$tmp/eightsleepctl-old"
perl -pi -e 's#"https://(client-api|app-api|auth-api)\.8slp\.net#os.environ["EIGHTSLEEP_COMPAT_BASE"] + "/$1#g' "$tmp/eightsleepctl-old"
[ "$(grep -c 'EIGHTSLEEP_COMPAT_BASE' "$tmp/eightsleepctl-old")" -eq 4 ]
chmod +x "$tmp/eightsleepctl-old"

# The test refuses to write an eightsleepctl golden with a nonzero exit or
# no requests, so a broken interpreter cannot record a failure as the contract.
GOWORK=off go test ./internal/compat -run 'TestCallers$' -count=1 -args \
  -record-eightctl "$tmp/eightctl-old" -record-eightsleepctl "$tmp/eightsleepctl-old" \
  -record-python "$python"
echo "recorded goldens from eightctl $ref and $script (sha256 $script_sha256) with $python"
