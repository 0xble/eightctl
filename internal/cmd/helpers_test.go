package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/steipete/eightctl/internal/daemon"
)

func TestParseDays(t *testing.T) {
	got, err := parseDays("1, 2,,6")
	if err != nil {
		t.Fatalf("parseDays: %v", err)
	}
	if !reflect.DeepEqual(got, []int{1, 2, 6}) {
		t.Fatalf("days = %#v", got)
	}
	if _, err := parseDays("x"); err == nil {
		t.Fatalf("expected invalid day error")
	}
}

func TestParseSchedule(t *testing.T) {
	got, err := parseSchedule([]byte("schedule:\n  - time: \"07:30\"\n    action: temp\n    temperature: 68F\n"))
	if err != nil {
		t.Fatalf("parseSchedule: %v", err)
	}
	want := []daemon.ScheduleItem{{Time: "07:30", Action: "temp", Temperature: "68F"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("schedule = %#v, want %#v", got, want)
	}
	if _, err := parseSchedule([]byte("schedule: []")); err == nil {
		t.Fatalf("expected empty schedule error")
	}
	if _, err := parseSchedule([]byte("schedule: [")); err == nil {
		t.Fatalf("expected yaml error")
	}
}

func TestReadConfigSchedule(t *testing.T) {
	resetViper(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("schedule: []"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	viper.SetConfigFile(path)
	if err := viper.ReadInConfig(); err != nil {
		t.Fatalf("ReadInConfig: %v", err)
	}
	got, err := readConfigSchedule()
	if err != nil {
		t.Fatalf("readConfigSchedule: %v", err)
	}
	if string(got) != "schedule: []" {
		t.Fatalf("config data = %q", got)
	}
}

func TestDefaultPIDFile(t *testing.T) {
	if got := defaultPIDFile("/tmp/custom.pid"); got != "/tmp/custom.pid" {
		t.Fatalf("defaultPIDFile explicit = %q", got)
	}
	got := defaultPIDFile("")
	if !strings.HasSuffix(got, filepath.Join(".config", "eightctl", "daemon.pid")) {
		t.Fatalf("defaultPIDFile = %q", got)
	}
}

func TestMapKeysAndCurrentDate(t *testing.T) {
	if got := mapKeys(map[string]any{"b": 2, "a": 1}); !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("mapKeys = %#v", got)
	}
	if got := currentDate(); len(got) != len("2006-01-02") {
		t.Fatalf("currentDate = %q", got)
	}
}

func TestMoreTempArgBranches(t *testing.T) {
	tests := [][]string{
		{"--side"},
		{"--target-user-id"},
		{"--", "1", "2"},
		{"1", "2"},
	}
	for _, args := range tests {
		if _, _, _, _, err := parseTempCommandArgs(args); err == nil {
			t.Fatalf("parseTempCommandArgs(%v): expected error", args)
		}
	}
	if !isNegativeTempCandidate("-.5") {
		t.Fatalf("-.5 should be a negative temp candidate")
	}
	if isNegativeTempCandidate("-x") {
		t.Fatalf("-x should not be a negative temp candidate")
	}
}
