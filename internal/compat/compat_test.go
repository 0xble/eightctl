// Package compat_test replays every eightctl command shape and every
// eightsleepctl command against the fake Eight Sleep and compares the result
// with golden output recorded from the pre-toolkit programs: eightctl at
// origin/main a2b8291 (0.2.8-0xble.0.1.0) and the eightsleepctl script.
//
// For each case it compares the exit code, the exact provider requests
// (method, host, path, sorted query), the stdout JSON or text, and the error
// message. Wall-clock values, the fake's address and the sandbox path are
// normalised.
//
// Re-record with internal/compat/record.sh.
package compat_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/0xble/toolkit"
	"github.com/0xble/toolkit/cli"

	"github.com/0xble/eightsleep/internal/eightfake"
	"github.com/0xble/eightsleep/ops"
)

var (
	recordEightctl      = flag.String("record-eightctl", "", "path to the pre-toolkit eightctl; rewrite its goldens")
	recordEightsleepctl = flag.String("record-eightsleepctl", "", "path to the eightsleepctl script; rewrite its goldens")
)

func TestMain(m *testing.M) {
	// Every program runs in UTC, so default dates agree between the old
	// programs and the replay whatever the machine's zone.
	time.Local = time.UTC
	os.Exit(m.Run())
}

const (
	eightctl      = "eightctl"
	eightsleepctl = "eightsleepctl"
)

type tcase struct {
	name string
	// caller names who runs this invocation; see docs/compatibility.md.
	caller string
	// old is the program the golden was recorded from.
	old  string
	args []string
	// newArgs, when set, is the eightsleep invocation of an eightsleepctl
	// command; otherwise eightsleep runs args.
	newArgs []string
	fake    func(*eightfake.Server)
	// noConfig runs without a config file; userID puts user_id in it.
	noConfig bool
	userID   bool
	// change, when set, is a documented intentional difference: stdout and
	// the error message are not compared. Exit code and requests still are,
	// unless exit or requests below say otherwise.
	change string
	// exit is the new exit code when it moved to the family table (C1).
	exit int
	// requests documents why the provider requests differ, and skips them.
	requests string
}

var (
	failGet = func(host, path string, status int) func(*eightfake.Server) {
		return func(s *eightfake.Server) { s.Fail = map[string]int{"GET " + host + " " + path: status} }
	}
	solo = func(s *eightfake.Server) { s.Solo = true }
)

var cases = []tcase{
	// Reads, in each output format.
	{name: "status", args: []string{"status"}},
	{name: "status-json", args: []string{"status", "--output", "json"}},
	{name: "status-side", args: []string{"status", "--side", "left", "--output", "json"}},
	{name: "status-all-csv", args: []string{"--output", "csv", "status", "--all-sides"}},
	{name: "status-unknown-user", args: []string{"status", "--target-user-id", "u9", "--output", "json"}},
	{name: "status-solo", args: []string{"status"}, fake: solo},
	{name: "status-no-household", args: []string{"status", "--output", "json"}, fake: failGet("client-api", "/v1/users/u2", 500)},
	{name: "whoami", args: []string{"whoami"}},
	{name: "whoami-offline", args: []string{"--user-id", "u1", "whoami"}, noConfig: true},
	{name: "version", args: []string{"version"}, change: "the version is the release tag"},
	{name: "version-flag", args: []string{"--version"}, change: "the version is the release tag"},
	{name: "tracks", args: []string{"tracks"}},
	{name: "tracks-json", args: []string{"tracks", "--output", "json"}},
	{name: "feats-csv", args: []string{"feats", "--output", "csv"}},
	{name: "alarm-list", args: []string{"alarm", "list"}, change: "a set sound prints its ID; eightctl printed a pointer address"},
	{name: "alarm-list-json", args: []string{"alarm", "list", "--output", "json"}},
	{name: "alarm-list-fallback", args: []string{"alarm", "list", "--output", "json"}, fake: failGet("client-api", "/v1/users/u1/alarms", 404)},
	{name: "alarm-list-fields", args: []string{"--fields", "id,time", "alarm", "list", "--output", "json"},
		change: "--fields filters --json output only (C4)"},
	{name: "presence-window", args: []string{"presence", "--from", "2026-10-04", "--to", "2026-10-05", "--output", "json"}},
	{name: "presence-default", args: []string{"presence"}},
	{name: "schedule", args: []string{"schedule", "list", "--output", "json"}},
	{name: "schedule-none", args: []string{"schedule", "list"}, fake: func(s *eightfake.Server) { s.NoSchedule = true }},
	{name: "sleep-day", args: []string{"sleep", "day", "--date", "2026-10-04"}},
	{name: "sleep-day-json", args: []string{"sleep", "day", "--date", "2026-10-04", "--output", "json"}},
	{name: "sleep-day-today", args: []string{"sleep", "day", "--output", "json"}},
	{name: "sleep-range", args: []string{"sleep", "range", "--from", "2026-10-03", "--to", "2026-10-04", "--output", "json"}},
	{name: "nap-status", args: []string{"tempmode", "nap", "status", "--output", "json"}},
	{name: "hotflash-status", args: []string{"tempmode", "hotflash", "status"}},
	{name: "temp-events", args: []string{"tempmode", "events", "--from", "2026-10-01", "--to", "2026-10-02", "--output", "json"}},
	{name: "audio-tracks", args: []string{"audio", "tracks"}},
	{name: "audio-categories", args: []string{"audio", "categories", "--output", "json"}},
	{name: "audio-state", args: []string{"audio", "state", "--output", "json"}},
	{name: "audio-next", args: []string{"audio", "next", "--output", "json"}},
	{name: "audio-favorites", args: []string{"audio", "favorites", "list", "--output", "json"}},
	{name: "base-info", args: []string{"base", "info", "--output", "json"}},
	{name: "base-presets", args: []string{"base", "presets"}},
	{name: "device-info", args: []string{"device", "info", "--output", "json"}},
	{name: "device-peripherals", args: []string{"device", "peripherals", "--output", "json"}},
	{name: "device-owner", args: []string{"device", "owner", "--output", "json"}},
	{name: "device-owner-fallback", args: []string{"device", "owner", "--output", "json"}, fake: func(s *eightfake.Server) {
		s.Fail = map[string]int{"GET client-api /v1/devices/d1/owner": 404, "GET app-api /v1/devices/d1/owner": 404}
	}},
	{name: "device-warranty", args: []string{"device", "warranty", "--output", "json"}},
	{name: "device-online", args: []string{"device", "online", "--output", "json"}},
	{name: "device-priming-tasks", args: []string{"device", "priming-tasks", "--output", "json"}},
	{name: "device-priming-schedule", args: []string{"device", "priming-schedule", "--output", "json"}},
	{name: "metrics-trends", args: []string{"metrics", "trends", "--from", "2026-10-01", "--to", "2026-10-02", "--output", "json"}},
	{name: "metrics-intervals", args: []string{"metrics", "intervals", "--id", "s1", "--output", "json"}},
	{name: "metrics-summary", args: []string{"metrics", "summary", "--output", "json"}},
	{name: "metrics-aggregate", args: []string{"metrics", "aggregate", "--output", "json"}},
	{name: "metrics-insights", args: []string{"metrics", "insights", "--output", "json"}},
	{name: "autopilot-details", args: []string{"autopilot", "details", "--output", "json"}},
	{name: "autopilot-history", args: []string{"autopilot", "history", "--output", "json"}},
	{name: "autopilot-recap", args: []string{"autopilot", "recap", "--output", "json"}},
	{name: "travel-trips", args: []string{"travel", "trips", "--output", "json"}},
	{name: "travel-plans", args: []string{"travel", "plans", "--trip", "trip1", "--output", "json"}},
	{name: "travel-tasks", args: []string{"travel", "tasks", "--plan", "p1", "--output", "json"}},
	{name: "travel-airports", args: []string{"travel", "airport-search", "--query", "SFO", "--output", "json"}},
	{name: "travel-flight", args: []string{"travel", "flight-status", "--flight", "UA1", "--output", "json"}},
	{name: "household-summary", args: []string{"household", "summary", "--output", "json"}},
	{name: "household-schedule", args: []string{"household", "schedule", "--output", "json"}},
	{name: "household-current-set", args: []string{"household", "current-set", "--output", "json"}},
	{name: "household-invitations", args: []string{"household", "invitations", "--output", "json"}},
	{name: "household-devices", args: []string{"household", "devices", "--output", "json"}},
	{name: "household-users", args: []string{"household", "users", "--output", "json"}},
	{name: "household-guests", args: []string{"household", "guests", "--output", "json"}},
	{name: "household-guests-missing", args: []string{"household", "guests", "--output", "json"},
		fake: failGet("app-api", "/v1/household/users/u1/guests", 404)},
	{name: "away-status", args: []string{"away", "status"}},
	{name: "away-status-side", args: []string{"away", "status", "--side", "right", "--output", "json"}},

	// Writes.
	{name: "on", args: []string{"on"}},
	{name: "off-side", args: []string{"off", "--side", "left"}},
	{name: "on-user", args: []string{"on", "--target-user-id", "u2"}},
	{name: "temp", args: []string{"temp", "20"}},
	{name: "temp-fahrenheit-user", args: []string{"temp", "68F", "--target-user-id", "u2"}},
	{name: "temp-negative-dashdash", args: []string{"temp", "--side", "right", "--", "-40"}},
	{name: "temp-negative", args: []string{"temp", "-40", "--side", "right"}, exit: 2, requests: "refused by the parser before any request",
		change: "a negative level needs -- before it (temp --side right -- -40): the toolkit's parser reads -40 as flags"},
	{name: "alarm-create", args: []string{"alarm", "create", "--time", "07:30", "--days", "1,2,3"}},
	{name: "alarm-create-full", args: []string{"alarm", "create", "--time", "06:00", "--days", "0", "--disabled", "--no-vibration", "--sound", "chime"}},
	{name: "alarm-update", args: []string{"alarm", "update", "a1", "--enabled=false", "--time", "06:45"}},
	{name: "alarm-delete", args: []string{"alarm", "delete", "a2"}},
	{name: "alarm-snooze", args: []string{"alarm", "snooze", "a1"}},
	{name: "alarm-dismiss", args: []string{"alarm", "dismiss", "a1"}},
	{name: "alarm-dismiss-all", args: []string{"alarm", "dismiss-all"}, requests: "C11: PUT on the app API instead of POST on the client API"},
	{name: "alarm-vibration-test", args: []string{"alarm", "vibration-test"}},
	{name: "away-on", args: []string{"away", "on"}},
	{name: "away-off-both", args: []string{"away", "off", "--both"}},
	{name: "away-on-side", args: []string{"away", "on", "--side", "left"}},
	{name: "away-on-quiet", args: []string{"--quiet", "away", "on"}},
	{name: "nap-on", args: []string{"tempmode", "nap", "on"}},
	{name: "nap-extend", args: []string{"tempmode", "nap", "extend"}},
	{name: "hotflash-off", args: []string{"tempmode", "hotflash", "off"}},
	{name: "audio-play", args: []string{"audio", "play", "--track", "t1"}},
	{name: "audio-pause", args: []string{"audio", "pause"}},
	{name: "audio-seek", args: []string{"audio", "seek", "--position", "5000"}},
	{name: "audio-volume-zero", args: []string{"audio", "volume", "--level", "0"}},
	{name: "audio-pair", args: []string{"audio", "pair"}},
	{name: "audio-favorite-add", args: []string{"audio", "favorites", "add", "--track", "t1"}},
	{name: "audio-favorite-remove", args: []string{"audio", "favorites", "remove", "--track", "t1"}},
	{name: "base-angle", args: []string{"base", "angle", "--head", "10", "--foot", "5"}},
	{name: "base-preset-run", args: []string{"base", "preset-run", "--name", "flat"}},
	{name: "base-test", args: []string{"base", "test"}},
	{name: "level-suggestions-off", args: []string{"autopilot", "level-suggestions", "--enabled=false"}},
	{name: "snore-mitigation", args: []string{"autopilot", "snore-mitigation"}},
	{name: "travel-create-trip", args: []string{"travel", "create-trip", "--destination", "Tokyo", "--trip-timezone", "Asia/Tokyo"}},
	{name: "travel-delete-trip", args: []string{"travel", "delete-trip", "--trip", "trip1"}},
	{name: "travel-create-plan", args: []string{"travel", "create-plan", "--trip", "trip1", "--name", "Adjust"}},
	{name: "travel-update-plan", args: []string{"travel", "update-plan", "--plan", "p1", "--date", "2026-10-09"}},
	{name: "logout", args: []string{"logout"}},

	// Error paths.
	{name: "err-no-credentials", args: []string{"status"}, noConfig: true, exit: 5},
	{name: "err-config-missing", args: []string{"--config", "/nonexistent/eightsleep.yaml", "status"}, noConfig: true},
	{name: "err-token-refused", args: []string{"alarm", "list"}, exit: 5, fake: func(s *eightfake.Server) {
		s.Fail = map[string]int{"POST auth-api /v1/tokens": 401}
	}},
	{name: "err-provider-500", args: []string{"status", "--side", "left"}, fake: failGet("app-api", "/v1/users/u1/temperature", 500)},
	{name: "err-not-found", args: []string{"metrics", "intervals", "--id", "s1"}, exit: 3, fake: func(s *eightfake.Server) {
		s.Fail = map[string]int{"GET client-api /v1/users/u1/intervals/s1": 404, "GET app-api /v1/users/u1/intervals/s1": 404}
	}},
	{name: "err-side-unknown", args: []string{"status", "--side", "middle"}, exit: 2},
	{name: "err-side-and-user", args: []string{"on", "--side", "left", "--target-user-id", "u2"}, exit: 2},
	{name: "err-all-sides-conflict", args: []string{"status", "--all-sides", "--side", "left"}, exit: 2},
	{name: "err-both-conflict", args: []string{"away", "on", "--both", "--side", "left"}, exit: 2},
	{name: "err-temp-invalid", args: []string{"temp", "warm"}, exit: 2},
	{name: "err-temp-missing", args: []string{"temp"}, exit: 2, change: "a parse error prints one error line (C2)"},
	{name: "err-alarm-create", args: []string{"alarm", "create"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-alarm-update-empty", args: []string{"alarm", "update", "a1"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-favorite-track", args: []string{"audio", "favorites", "add"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-create-trip-empty", args: []string{"travel", "create-trip"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-sleep-range-missing", args: []string{"sleep", "range"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-presence-date", args: []string{"presence", "--from", "2026-13-01"}, exit: 2, requests: "validated before credentials are checked"},
	{name: "err-daemon-no-schedule", args: []string{"daemon", "--dry-run"}, exit: 2},
	{name: "err-unknown-command", args: []string{"definitely-not-a-command"}, exit: 2, change: "a parse error prints one error line (C2)"},
	{name: "err-unknown-flag", args: []string{"status", "--definitely-not-a-flag"}, exit: 2, change: "a parse error prints one error line (C2)"},
	{name: "err-bare", args: []string{}, exit: 2, change: "a bare invocation is a usage error instead of help (C1)"},

	// eightsleepctl, run as the script with --output json and as the
	// eightsleep equivalent with --json (docs/compatibility.md#eightsleepctl).
	{name: "esc-whoami", old: eightsleepctl, userID: true, args: []string{"whoami"}, newArgs: []string{"whoami", "--json"},
		change: "token_expires_at is present only when a token is cached", requests: "eightsleep answers a configured user ID without requesting a token"},
	{name: "esc-alarm-list", old: eightsleepctl, userID: true, args: []string{"alarm", "list"}, newArgs: []string{"alarm", "list", "--json"},
		change: "rows {id,time,enabled,days,vibration,sound} instead of the raw alarm objects", requests: "eightctl's alarm list reads the client API, the script the app API"},
	{name: "esc-alarm-active", old: eightsleepctl, userID: true, args: []string{"alarm", "active"}, newArgs: []string{"alarm", "active", "--json"}},
	{name: "esc-dismiss-all", old: eightsleepctl, userID: true, args: []string{"alarm", "dismiss-all"}, newArgs: []string{"alarm", "dismiss-all", "--json"}},
	{name: "esc-dismiss-all-fallback", old: eightsleepctl, userID: true, args: []string{"alarm", "dismiss-all"}, newArgs: []string{"alarm", "dismiss-all", "--json"},
		fake: func(s *eightfake.Server) { s.DismissAllStatus = 405 }},
	{name: "esc-dismiss-all-dry-run", old: eightsleepctl, userID: true, args: []string{"alarm", "dismiss-all", "--dry-run"},
		newArgs: []string{"alarm", "dismiss-all", "--dry-run", "--json"},
		change:  "fallback names the route the fallback really uses (POST each active alarm's dismiss) instead of a routines PUT"},
	{name: "esc-presence", old: eightsleepctl, userID: true, args: []string{"presence"}, newArgs: []string{"presence", "detail", "--json"}},
	{name: "esc-presence-stale", old: eightsleepctl, userID: true, args: []string{"presence"}, newArgs: []string{"presence", "detail", "--json"},
		fake: func(s *eightfake.Server) { s.StaleSignals = true }},
}

// golden is what one case produced.
type golden struct {
	Caller     string              `json:"caller,omitempty"`
	Program    string              `json:"program"`
	Args       []string            `json:"args"`
	NewArgs    []string            `json:"new_args,omitempty"`
	Exit       int                 `json:"exit"`
	StdoutJSON any                 `json:"stdout_json,omitempty"`
	StdoutText string              `json:"stdout_text,omitempty"`
	Error      string              `json:"error,omitempty"`
	Requests   []eightfake.Request `json:"requests"`
	// Writes are the bodies of the applied changes.
	Writes []eightfake.Write `json:"writes,omitempty"`
	Change string            `json:"change,omitempty"`
}

func TestCallers(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range cases {
		if seen[c.name] {
			t.Fatalf("duplicate case %s", c.name)
		}
		seen[c.name] = true
		if c.old == "" {
			c.old = eightctl
		}
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join("testdata", c.name+".json")
			bin := map[string]string{eightctl: *recordEightctl, eightsleepctl: *recordEightsleepctl}[c.old]
			if *recordEightctl != "" || *recordEightsleepctl != "" {
				if bin == "" {
					t.Skipf("no %s to record from", c.old)
				}
				g := runOld(t, bin, c)
				b, _ := json.MarshalIndent(g, "", "  ")
				if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			b, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("no golden for %s; record it with internal/compat/record.sh: %v", c.name, err)
			}
			var want golden
			if err := json.Unmarshal(b, &want); err != nil {
				t.Fatal(err)
			}
			compare(t, c, want, runNew(t, c))
		})
	}
}

func compare(t *testing.T, c tcase, want, got golden) {
	t.Helper()
	wantExit := want.Exit
	if c.exit != 0 {
		wantExit = c.exit
	}
	if got.Exit != wantExit {
		t.Errorf("exit %d, want %d (old program %d)", got.Exit, wantExit, want.Exit)
	}
	if c.requests == "" && !reflect.DeepEqual(got.Requests, want.Requests) {
		t.Errorf("provider requests differ:\n new %v\n old %v", got.Requests, want.Requests)
	}
	if c.requests == "" && !reflect.DeepEqual(roundTrip(got.Writes), roundTrip(want.Writes)) {
		g, _ := json.Marshal(got.Writes)
		w, _ := json.Marshal(want.Writes)
		t.Errorf("write bodies differ:\n new %s\n old %s", g, w)
	}
	if c.change != "" {
		return
	}
	if !reflect.DeepEqual(roundTrip(got.StdoutJSON), roundTrip(want.StdoutJSON)) {
		g, _ := json.Marshal(got.StdoutJSON)
		w, _ := json.Marshal(want.StdoutJSON)
		t.Errorf("stdout JSON differs:\n new %s\n old %s", g, w)
	}
	if got.StdoutText != want.StdoutText {
		t.Errorf("stdout text differs:\n new %q\n old %q", got.StdoutText, want.StdoutText)
	}
	if want.Error != "" && got.Error != want.Error {
		t.Errorf("error differs:\n new %q\n old %q", got.Error, want.Error)
	}
}

func roundTrip(v any) any {
	b, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(b, &out)
	return out
}

// sandbox is the private home every run gets: a config file with fixture
// credentials (unless the case has none), UTC, and the fake's address.
func sandbox(t *testing.T, c tcase, fake *eightfake.Server) (map[string]string, string) {
	t.Helper()
	home := t.TempDir()
	if !c.noConfig {
		dir := filepath.Join(home, ".config", "eightctl")
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		cfg := "email: ada@example.invalid\npassword: fixture-password\ntimezone: UTC\nclient_id: fixture-client\nclient_secret: fixture-secret\n"
		if c.userID {
			cfg += "user_id: u1\n"
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(cfg), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return map[string]string{
		"HOME": home, "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "XDG_STATE_HOME": filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME": filepath.Join(home, ".cache"), "XDG_DATA_HOME": filepath.Join(home, ".local", "share"),
		"TZ": "UTC", "USER": "compat", "EIGHTSLEEP_COMPAT_BASE": fake.URL,
	}, home
}

func newFake(c tcase) *eightfake.Server {
	fake := eightfake.New()
	if c.fake != nil {
		c.fake(fake)
	}
	return fake
}

func runOld(t *testing.T, bin string, c tcase) golden {
	fake := newFake(c)
	defer fake.Close()
	env, home := sandbox(t, c, fake)
	args := c.args
	cmd := exec.Command(bin, args...)
	if c.old == eightsleepctl {
		args = append([]string{"--config", filepath.Join(home, ".config", "eightctl", "config.yaml"), "--output", "json"}, c.args...)
		cmd = exec.Command("python3", append([]string{"-I", bin}, args...)...)
	}
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr, cmd.Stdin = &stdout, &stderr, strings.NewReader("")
	code := 0
	if err := cmd.Run(); err != nil {
		ee, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = ee.ExitCode()
	}
	errLine := ""
	if m := regexp.MustCompile(`(?m)^Error: (.*)$`).FindStringSubmatch(stderr.String()); m != nil {
		errLine = m[1]
	}
	return result(c, code, stdout.String(), errLine, fake, home)
}

func runNew(t *testing.T, c tcase) golden {
	fake := newFake(c)
	defer fake.Close()
	env, home := sandbox(t, c, fake)
	for k, v := range env {
		t.Setenv(k, v)
	}
	for _, p := range []string{"EIGHTCTL_", "EIGHTSLEEP_"} {
		for _, k := range []string{"EMAIL", "PASSWORD", "USER_ID", "CLIENT_ID", "CLIENT_SECRET", "TIMEZONE", "OUTPUT", "CONFIG", "QUIET", "CONFIG_QUIET", "VERBOSE"} {
			t.Setenv(p+k, "")
		}
	}
	hosts := fake.Hosts()
	b := &ops.Backend{Globals: &ops.Globals{}, Hosts: &hosts, Stderr: &bytes.Buffer{}, Version: "dev"}
	args := c.args
	if c.newArgs != nil {
		args = c.newArgs
	}
	var stdout, stderr bytes.Buffer
	o := toolkit.CLIOptions(ops.Options(b))
	o.Stdin, o.Stdout, o.Stderr = strings.NewReader(""), &stdout, &stderr
	code := cli.Run(context.Background(), ops.New("dev", b), o, args)
	errLine := ""
	if m := regexp.MustCompile(`(?m)^error: (.*)$`).FindStringSubmatch(stderr.String()); m != nil {
		errLine = m[1]
	}
	if c.newArgs == nil && stdout.Len() == 0 && stderr.Len() > 0 && strings.HasPrefix(stderr.String(), "{") {
		var env struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(stderr.Bytes(), &env) == nil {
			errLine = env.Error.Message
		}
	}
	return result(c, code, stdout.String(), errLine, fake, home)
}

var (
	rfc3339 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})$`)
	port    = regexp.MustCompile(`http://127\.0\.0\.1:\d+`)
	// recent is a timestamp the fake derived from the wall clock.
	recent = regexp.MustCompile(`<(today|yesterday)>T\d{2}:\d{2}:\d{2}(\.\d+)?Z`)
)

func result(c tcase, code int, stdout, errLine string, fake *eightfake.Server, home string) golden {
	reqs := fake.Recorded()
	if reqs == nil {
		reqs = []eightfake.Request{}
	}
	today := time.Now().UTC().Format("2006-01-02")
	yesterday := time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")
	clean := func(s string) string {
		s = strings.ReplaceAll(s, home, "<home>")
		s = port.ReplaceAllString(s, "<fake>")
		s = strings.ReplaceAll(s, today, "<today>")
		s = strings.ReplaceAll(s, yesterday, "<yesterday>")
		return recent.ReplaceAllString(s, "<recent>")
	}
	for i := range reqs {
		reqs[i].Query = clean(reqs[i].Query)
	}
	writes, _ := fake.Snapshot().([]eightfake.Write)
	if len(writes) == 0 {
		writes = nil
	}
	for i := range writes {
		writes[i].Body = normalise(writes[i].Body, clean)
	}
	g := golden{Caller: c.caller, Program: c.old, Args: c.args, NewArgs: c.newArgs, Exit: code, Requests: reqs,
		Writes: writes, Change: c.change, Error: clean(errLine)}
	if g.Program == "" {
		g.Program = eightctl
	}
	var v any
	if err := json.Unmarshal([]byte(stdout), &v); err == nil && strings.TrimSpace(stdout) != "" {
		g.StdoutJSON = normalise(v, clean)
	} else {
		g.StdoutText = clean(stdout)
	}
	return g
}

// normalise masks values that depend on the wall clock: timestamps the fake
// derives from now, token expiries and signal ages.
func normalise(v any, clean func(string) string) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			switch {
			case k == "age_seconds" && val != nil:
				x[k] = "<age>"
			case (k == "timestamp" || k == "token_expires_at") && isTime(val):
				x[k] = "<time>"
			default:
				x[k] = normalise(val, clean)
			}
		}
	case []any:
		for i := range x {
			x[i] = normalise(x[i], clean)
		}
	case string:
		return clean(x)
	}
	return v
}

func isTime(v any) bool {
	s, ok := v.(string)
	return ok && rfc3339.MatchString(s)
}

// TestEveryCaseHasAGolden keeps the table and testdata in step.
func TestEveryCaseHasAGolden(t *testing.T) {
	if *recordEightctl != "" || *recordEightsleepctl != "" {
		t.Skip("recording")
	}
	files, _ := filepath.Glob(filepath.Join("testdata", "*.json"))
	if len(files) != len(cases) {
		t.Errorf("%d goldens for %d cases", len(files), len(cases))
	}
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".json")
		found := false
		for _, c := range cases {
			found = found || c.name == name
		}
		if !found {
			t.Errorf("golden %s has no case", name)
		}
	}
}
