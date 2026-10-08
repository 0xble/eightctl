package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, data string, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(data), mode); err != nil {
		t.Fatal(err)
	}
}

func envOf(vars map[string]string) func(string) string { return func(k string) string { return vars[k] } }

func TestLoadReadsConfigAndEnv(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".config", "eightctl", "config.yaml"),
		"email: file@example.invalid\npassword: file-pass\ntimezone: America/New_York\noutput: JSON\nquiet: true\n", 0o600)
	s, err := Load(Flags{Password: "flag-pass"}, home, envOf(map[string]string{"EIGHTCTL_EMAIL": "env@example.invalid"}))
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "env@example.invalid" || s.Password != "flag-pass" || s.Timezone != "America/New_York" || s.Output != "json" {
		t.Fatalf("precedence is flag, env, file: %+v", s)
	}
	if !s.AwayQuiet || s.Quiet {
		t.Fatalf("the quiet key silences away output only: %+v", s)
	}
	if s.Path == "" || s.Insecure() {
		t.Fatalf("path %q, insecure %v", s.Path, s.Insecure())
	}
}

func TestLoadDefaultsWhenConfigMissing(t *testing.T) {
	s, err := Load(Flags{}, t.TempDir(), envOf(nil))
	if err != nil {
		t.Fatal(err)
	}
	if s.Path != "" || s.Timezone != "local" || s.Output != "table" {
		t.Fatalf("defaults: %+v", s)
	}
}

func TestLoadPrefersEightsleepPathAndPrefix(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".config", "eightctl", "config.yaml"), "email: old@example.invalid\n", 0o600)
	write(t, filepath.Join(home, ".config", "eightsleep", "config.yaml"), "email: new@example.invalid\n", 0o600)
	s, err := Load(Flags{}, home, envOf(map[string]string{"EIGHTCTL_TIMEZONE": "UTC", "EIGHTSLEEP_TIMEZONE": "Asia/Tokyo"}))
	if err != nil {
		t.Fatal(err)
	}
	if s.Email != "new@example.invalid" || s.Timezone != "Asia/Tokyo" {
		t.Fatalf("eightsleep path and prefix win: %+v", s)
	}
}

func TestLoadConfigSelectedByEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.yaml")
	write(t, path, "user_id: from-env-file\n", 0o600)
	s, err := Load(Flags{}, t.TempDir(), envOf(map[string]string{"EIGHTCTL_CONFIG": path}))
	if err != nil || s.UserID != "from-env-file" {
		t.Fatalf("user %q, err %v", s.UserID, err)
	}
}

func TestLoadReportsSelectedFileErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.yaml")
	if _, err := Load(Flags{}, t.TempDir(), envOf(map[string]string{"EIGHTCTL_CONFIG": missing})); err == nil {
		t.Fatal("a missing file selected by the environment must fail")
	}
	if _, err := Load(Flags{Config: missing}, t.TempDir(), envOf(nil)); err == nil {
		t.Fatal("a missing --config file must fail")
	}
	bad := filepath.Join(t.TempDir(), "bad.yaml")
	write(t, bad, "email: [unclosed\n", 0o600)
	if _, err := Load(Flags{Config: bad}, t.TempDir(), envOf(nil)); err == nil || !strings.Contains(err.Error(), "read config") {
		t.Fatalf("malformed YAML: %v", err)
	}
}

func TestInsecure(t *testing.T) {
	home := t.TempDir()
	write(t, filepath.Join(home, ".config", "eightctl", "config.yaml"), "email: a@example.invalid\n", 0o644)
	s, err := Load(Flags{}, home, envOf(nil))
	if err != nil || !s.Insecure() {
		t.Fatalf("a 0644 config is insecure: %v %v", s.Insecure(), err)
	}
}
