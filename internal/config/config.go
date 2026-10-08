// Package config merges eightsleep's settings: root flags, then
// EIGHTSLEEP_ and EIGHTCTL_ environment variables, then the YAML config file,
// the precedence eightctl's viper setup had.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Prefixes are the environment variable prefixes, most preferred first.
// EIGHTCTL_ is the name every existing setup uses.
var Prefixes = []string{"EIGHTSLEEP_", "EIGHTCTL_"}

// Flags are the root flag values. Empty means not given.
type Flags struct {
	Config       string
	Email        string
	Password     string
	UserID       string
	ClientID     string
	ClientSecret string
	Timezone     string
	Output       string
	Verbose      bool
	Quiet        bool
}

// Settings are the merged values.
type Settings struct {
	Email        string
	Password     string
	UserID       string
	ClientID     string
	ClientSecret string
	// Timezone is an IANA zone name or "local".
	Timezone string
	// Output is the human output format: table, json or csv.
	Output  string
	Verbose bool
	// Quiet suppresses the config-file notice.
	Quiet bool
	// AwayQuiet suppresses the away on/off confirmation line. eightctl read
	// it from EIGHTCTL_QUIET or the config's quiet key.
	AwayQuiet bool
	// Path is the config file that was read, or empty.
	Path string
	// Data is that file's contents, for the daemon's schedule.
	Data []byte
	// Mode is the file's permission bits.
	Mode fs.FileMode
}

// file is the config file. Unknown keys, such as the daemon's schedule, are
// ignored here.
type file struct {
	Email        string `yaml:"email"`
	Password     string `yaml:"password"`
	UserID       string `yaml:"user_id"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Timezone     string `yaml:"timezone"`
	Output       string `yaml:"output"`
	Verbose      bool   `yaml:"verbose"`
	Quiet        bool   `yaml:"quiet"`
}

// Load reads the settings. home is the user's home directory and getenv reads
// the environment.
func Load(f Flags, home string, getenv func(string) string) (Settings, error) {
	env := func(key string) string {
		for _, p := range Prefixes {
			if v := getenv(p + key); v != "" {
				return v
			}
		}
		return ""
	}
	path, explicit := f.Config, f.Config != ""
	if path == "" {
		if path = env("CONFIG"); path != "" {
			explicit = true
		}
	}
	if path == "" {
		path = DefaultPath(home)
	}
	var cfg file
	s := Settings{}
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Settings{}, fmt.Errorf("config: read config: %s: %w", path, err)
		}
		s.Path, s.Data = path, data
		if info, err := os.Stat(path); err == nil {
			s.Mode = info.Mode().Perm()
		}
	case explicit || !errors.Is(err, fs.ErrNotExist):
		return Settings{}, fmt.Errorf("config: read config: %w", err)
	}
	pick := func(flag, key, conf, def string) string {
		for _, v := range []string{flag, env(key), conf} {
			if v != "" {
				return v
			}
		}
		return def
	}
	truth := func(flag bool, key string, conf bool) bool {
		if flag {
			return true
		}
		if v := env(key); v != "" {
			b, _ := strconv.ParseBool(v)
			return b
		}
		return conf
	}
	s.Email = pick(f.Email, "EMAIL", cfg.Email, "")
	s.Password = pick(f.Password, "PASSWORD", cfg.Password, "")
	s.UserID = pick(f.UserID, "USER_ID", cfg.UserID, "")
	s.ClientID = pick(f.ClientID, "CLIENT_ID", cfg.ClientID, "")
	s.ClientSecret = pick(f.ClientSecret, "CLIENT_SECRET", cfg.ClientSecret, "")
	s.Timezone = pick(f.Timezone, "TIMEZONE", cfg.Timezone, "local")
	s.Output = strings.ToLower(pick(f.Output, "OUTPUT", cfg.Output, "table"))
	s.Verbose = truth(f.Verbose, "VERBOSE", cfg.Verbose)
	s.Quiet = truth(f.Quiet, "CONFIG_QUIET", false)
	s.AwayQuiet = truth(false, "QUIET", cfg.Quiet)
	return s, nil
}

// DefaultPath is ~/.config/eightsleep/config.yaml when it exists, else the
// eightctl path every existing setup uses.
func DefaultPath(home string) string {
	for _, name := range []string{"eightsleep", "eightctl"} {
		for _, file := range []string{"config.yaml", "config.yml"} {
			p := filepath.Join(home, ".config", name, file)
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return filepath.Join(home, ".config", "eightctl", "config.yaml")
}

// Insecure reports whether the config file is readable by group or others.
func (s Settings) Insecure() bool { return s.Path != "" && s.Mode&0o077 != 0 }
