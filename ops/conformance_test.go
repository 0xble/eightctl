package ops_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0xble/toolkit/toolkittest"

	"github.com/0xble/eightsleep/ops"
)

// TestConformance runs the toolkit conformance kit against the fake: metadata,
// OpenAPI and MCP match the registry, CLI, HTTP and MCP agree on every case,
// previews and refused applies change nothing, immediate writes apply on the
// CLI and preview with --dry-run, the deletes need confirm on HTTP and MCP,
// and the served default refuses applied writes.
func TestConformance(t *testing.T) {
	toolkittest.Run(t, toolkittest.Suite{
		New: func(t testing.TB) toolkittest.Fixture {
			f := newFixture(t)
			return toolkittest.Fixture{Registry: f.reg, State: f.fake.Snapshot}
		},
		Options: ops.Options(&ops.Backend{Globals: &ops.Globals{}}),
		Cases:   conformanceCases(t),
	})
}

// conformanceCases is one call of every operation, as wire input and as CLI
// arguments.
func conformanceCases(t testing.TB) map[string]toolkittest.Case {
	in := func(kv ...any) map[string]any {
		m := map[string]any{}
		for i := 0; i+1 < len(kv); i += 2 {
			m[kv[i].(string)] = kv[i+1]
		}
		return m
	}
	a := func(args ...string) []string { return args }
	cases := map[string]toolkittest.Case{
		"on":                          {Input: in("side", "left"), Args: a("on", "--side", "left")},
		"off":                         {Input: in(), Args: a("off")},
		"temp":                        {Input: in("value", "68F", "target_user_id", "u2"), Args: a("temp", "68F", "--target-user-id", "u2")},
		"status":                      {Input: in("all_sides", true), Args: a("status", "--all-sides")},
		"alarm.create":                {Input: in("time", "07:30", "days", []any{1, 2}), Args: a("alarm", "create", "--time", "07:30", "--days", "1,2")},
		"alarm.update":                {Input: in("id", "a1", "enabled", false), Args: a("alarm", "update", "a1", "--enabled=false")},
		"alarm.delete":                {Input: in("id", "a2"), Args: a("alarm", "delete", "a2")},
		"alarm.snooze":                {Input: in("id", "a1"), Args: a("alarm", "snooze", "a1")},
		"alarm.dismiss":               {Input: in("id", "a1"), Args: a("alarm", "dismiss", "a1")},
		"away.on":                     {Input: in("both", true), Args: a("away", "on", "--both")},
		"away.off":                    {Input: in("side", "right"), Args: a("away", "off", "--side", "right")},
		"away.status":                 {Input: in("target_user_id", "u2"), Args: a("away", "status", "--target-user-id", "u2")},
		"presence":                    {Input: in("from", "2026-10-04", "to", "2026-10-05"), Args: a("presence", "--from", "2026-10-04", "--to", "2026-10-05")},
		"presence.detail":             {Input: in("timezone", "UTC"), Args: a("--timezone", "UTC", "presence", "detail")},
		"sleep.day":                   {Input: in("date", "2026-10-04"), Args: a("sleep", "day", "--date", "2026-10-04")},
		"sleep.range":                 {Input: in("from", "2026-10-03", "to", "2026-10-04"), Args: a("sleep", "range", "--from", "2026-10-03", "--to", "2026-10-04")},
		"metrics.trends":              {Input: in("from", "2026-10-01", "to", "2026-10-02"), Args: a("metrics", "trends", "--from", "2026-10-01", "--to", "2026-10-02")},
		"metrics.intervals":           {Input: in("id", "s1"), Args: a("metrics", "intervals", "--id", "s1")},
		"tempmode.events":             {Input: in("from", "2026-10-01"), Args: a("tempmode", "events", "--from", "2026-10-01")},
		"audio.play":                  {Input: in("track", "t1"), Args: a("audio", "play", "--track", "t1")},
		"audio.seek":                  {Input: in("position", 1000), Args: a("audio", "seek", "--position", "1000")},
		"audio.volume":                {Input: in("level", 30), Args: a("audio", "volume", "--level", "30")},
		"audio.favorites.add":         {Input: in("track", "t1"), Args: a("audio", "favorites", "add", "--track", "t1")},
		"audio.favorites.remove":      {Input: in("track", "t1"), Args: a("audio", "favorites", "remove", "--track", "t1")},
		"base.angle":                  {Input: in("head", 10, "foot", 5), Args: a("base", "angle", "--head", "10", "--foot", "5")},
		"base.preset_run":             {Input: in("name", "flat"), Args: a("base", "preset-run", "--name", "flat")},
		"autopilot.level_suggestions": {Input: in("enabled", false), Args: a("autopilot", "level-suggestions", "--enabled=false")},
		"autopilot.snore_mitigation":  {Input: in(), Args: a("autopilot", "snore-mitigation")},
		"travel.plans":                {Input: in("trip", "trip1"), Args: a("travel", "plans", "--trip", "trip1")},
		"travel.tasks":                {Input: in("plan", "p1"), Args: a("travel", "tasks", "--plan", "p1")},
		"travel.airport_search":       {Input: in("query", "SFO"), Args: a("travel", "airport-search", "--query", "SFO")},
		"travel.flight_status":        {Input: in("flight", "UA1"), Args: a("travel", "flight-status", "--flight", "UA1")},
		"travel.create_trip":          {Input: in("destination", "Tokyo", "trip_timezone", "Asia/Tokyo"), Args: a("travel", "create-trip", "--destination", "Tokyo", "--trip-timezone", "Asia/Tokyo")},
		"travel.delete_trip":          {Input: in("trip", "trip1"), Args: a("travel", "delete-trip", "--trip", "trip1")},
		"travel.create_plan":          {Input: in("trip", "trip1", "name", "Adjust"), Args: a("travel", "create-plan", "--trip", "trip1", "--name", "Adjust")},
		"travel.update_plan":          {Input: in("plan", "p1", "date", "2026-10-09"), Args: a("travel", "update-plan", "--plan", "p1", "--date", "2026-10-09")},
	}
	// Every other operation takes no input: its case is its command words.
	for _, e := range newFixture(t).reg.Entries() {
		if _, ok := cases[e.Name]; ok || e.In != reflect.TypeFor[ops.NoInput]() {
			continue
		}
		cases[e.Name] = toolkittest.Case{Input: in(), Args: strings.Fields(e.CLI())}
	}
	return cases
}

// TestEveryOperationHasACase keeps the conformance table complete: a read
// without a case would skip its parity check.
func TestEveryOperationHasACase(t *testing.T) {
	f := newFixture(t)
	for _, e := range f.reg.Entries() {
		if e.In == reflect.TypeFor[ops.NoInput]() {
			continue
		}
		switch e.Name {
		case "on", "off", "temp", "status", "alarm.create", "alarm.update", "alarm.delete", "alarm.snooze", "alarm.dismiss",
			"away.on", "away.off", "away.status", "presence", "presence.detail", "sleep.day", "sleep.range", "metrics.trends",
			"metrics.intervals", "tempmode.events", "audio.play", "audio.seek", "audio.volume", "audio.favorites.add",
			"audio.favorites.remove", "base.angle", "base.preset_run", "autopilot.level_suggestions", "autopilot.snore_mitigation",
			"travel.plans", "travel.tasks", "travel.airport_search", "travel.flight_status", "travel.create_trip",
			"travel.delete_trip", "travel.create_plan", "travel.update_plan":
		default:
			t.Errorf("%s takes input but has no conformance case", e.Name)
		}
	}
}
