package ops

import (
	"testing"

	"github.com/0xble/eightsleep/internal/client"
)

func TestAPIZone(t *testing.T) {
	b := &Backend{LocalZone: func() string { return "Europe/Paris" }}
	for _, c := range []struct{ input, configured, want string }{
		{"Asia/Tokyo", "UTC", "Asia/Tokyo"},
		{"", "America/New_York", "America/New_York"},
		{"", "local", "Europe/Paris"},
		{"LOCAL", "", "Europe/Paris"},
	} {
		if got := b.apiZone(c.input, c.configured); got != c.want {
			t.Errorf("apiZone(%q, %q) = %q, want %q", c.input, c.configured, got, c.want)
		}
	}
	unknown := &Backend{LocalZone: func() string { return "" }}
	if got := unknown.apiZone("", "local"); got == "" || got == "Local" {
		t.Fatalf("an unresolvable local zone must not reach the API as %q", got)
	}
}

func TestZoneinfoSuffix(t *testing.T) {
	if got := zoneinfoSuffix("/var/db/timezone/zoneinfo/America/Los_Angeles"); got != "America/Los_Angeles" {
		t.Fatal(got)
	}
	if got := zoneinfoSuffix("/etc/localtime"); got != "" {
		t.Fatal(got)
	}
}

func TestTargetListSuffix(t *testing.T) {
	for _, c := range []struct {
		targets []client.HouseholdUserTarget
		want    string
	}{
		{nil, ""},
		{[]client.HouseholdUserTarget{{UserID: "u1", Side: "left"}}, " for side left"},
		{[]client.HouseholdUserTarget{{UserID: "u1"}}, " for user u1"},
		{[]client.HouseholdUserTarget{{UserID: "u1", Side: "left"}, {UserID: "u2", Side: "right"}}, " for sides left, right"},
		{[]client.HouseholdUserTarget{{UserID: "u1", Side: "left"}, {UserID: "u2"}}, " for all discovered users"},
	} {
		if got := targetListSuffix(c.targets); got != c.want {
			t.Errorf("%v: %q, want %q", c.targets, got, c.want)
		}
	}
}

func TestDayBounds(t *testing.T) {
	from, to := dayBounds("2026-10-01", "2026-10-02")
	if from != "2026-10-01T00:00:00Z" || to != "2026-10-02T23:59:59Z" {
		t.Fatal(from, to)
	}
	if from, to = dayBounds("2026-10-01T05:00:00Z", ""); from != "2026-10-01T05:00:00Z" || to != "" {
		t.Fatal(from, to)
	}
}
