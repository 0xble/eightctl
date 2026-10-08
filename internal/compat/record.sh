#!/usr/bin/env bash
# Re-record the caller-compatibility goldens from the pre-toolkit programs.
# Usage: internal/compat/record.sh [git-ref] [eightsleepctl-script]
# The default ref is a2b8291, origin/main immediately before the rewrite, and
# the default script is the dotfiles source of ~/.local/bin/eightsleepctl.
# Each old program is copied to a temporary directory and changed in exactly
# one way: its Eight Sleep hosts read EIGHTSLEEP_COMPAT_BASE, so it reaches
# the test fake instead of the real service.
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."

ref="${1:-a2b8291}"
script="${2:-$HOME/dotfiles/dot_local/bin/executable_eightsleepctl}"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
mkdir -p "$tmp/src"
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

cp "$script" "$tmp/eightsleepctl-old"
perl -pi -e 's#"https://(client-api|app-api|auth-api)\.8slp\.net#os.environ["EIGHTSLEEP_COMPAT_BASE"] + "/$1#g' "$tmp/eightsleepctl-old"
[ "$(grep -c 'EIGHTSLEEP_COMPAT_BASE' "$tmp/eightsleepctl-old")" -eq 4 ]
chmod +x "$tmp/eightsleepctl-old"

GOWORK=off go test ./internal/compat -run 'TestCallers$' -count=1 -args \
  -record-eightctl "$tmp/eightctl-old" -record-eightsleepctl "$tmp/eightsleepctl-old"
echo "recorded goldens from eightctl $ref and $script"
