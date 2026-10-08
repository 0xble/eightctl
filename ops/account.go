package ops

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/0xble/toolkit/cli"
	"github.com/0xble/toolkit/op"
	"github.com/0xble/toolkit/output"
	"go.yaml.in/yaml/v3"

	"github.com/0xble/eightsleep/internal/daemon"
	"github.com/0xble/eightsleep/internal/tokencache"
)

// NoInput is the input of an operation without parameters.
type NoInput struct{}

// Whoami is the resolved user and, when a token is cached, its expiry.
type Whoami struct {
	UserID string `json:"user_id"`
	// TokenExpiresAt is the cached token's expiry, absent without one.
	TokenExpiresAt *time.Time `json:"token_expires_at,omitempty"`
}

// Version is the tool version.
type Version struct {
	Version string `json:"version"`
}

func registerAccount(reg *op.Registry, b *Backend) {
	op.Add(reg, op.Op[NoInput, Whoami]{
		Name: "whoami", Summary: "Show the configured or cached user ID", Effect: op.Read, MCP: true,
		Handler: func(ctx context.Context, req op.Request, _ NoInput) (Whoami, error) {
			s, err := b.load(req.Surface)
			if err != nil {
				return Whoami{}, err
			}
			// --user-id answers offline, without credentials.
			cl := b.newClient(s)
			if s.UserID == "" {
				if cl, _, err = b.client(req); err != nil {
					return Whoami{}, err
				}
			}
			if err := cl.EnsureUserID(ctx); err != nil {
				return Whoami{}, providerErr(err)
			}
			out := Whoami{UserID: cl.UserID}
			exp := cl.TokenExpiry()
			if exp.IsZero() {
				if cached, err := tokencache.Load(cl.Identity()); err == nil {
					exp = cached.ExpiresAt
				}
			}
			if !exp.IsZero() {
				exp = exp.UTC()
				out.TokenExpiresAt = &exp
			}
			return out, nil
		},
		Render: func(w io.Writer, o Whoami) error {
			_, err := fmt.Fprintf(w, "UserID: %s\n", o.UserID)
			return err
		},
	})
	op.Add(reg, op.Op[NoInput, Version]{
		Name: "version", Summary: "Print version", Effect: op.Read,
		Handler: func(context.Context, op.Request, NoInput) (Version, error) {
			return Version{Version: b.Version}, nil
		},
		Render: func(w io.Writer, v Version) error {
			_, err := fmt.Fprintln(w, v.Version)
			return err
		},
	})
}

// logoutCmd clears the local token cache. It does not revoke the token.
type logoutCmd struct {
	b *Backend
}

func (c *logoutCmd) Run(ctx *cli.Context) error {
	s, err := c.b.load(op.SurfaceCLI)
	if err != nil {
		return err
	}
	if err := tokencache.Clear(c.b.newClient(s).Identity()); err != nil {
		return op.Errorf(op.KindError, "logout_failed", "clear token: %v", err)
	}
	if ctx.JSON {
		return output.EncodeJSON(ctx.Stdout, map[string]bool{"cleared": true})
	}
	_, err = fmt.Fprintln(ctx.Stdout, "Logged out (token cache cleared)")
	return err
}

// daemonCmd runs the config file's schedule until signalled.
type daemonCmd struct {
	DryRun    bool   `name:"dry-run" help:"log actions without executing"`
	SyncState bool   `name:"sync-state" help:"(reserved) sync device state"`
	PIDFile   string `name:"pid-file" help:"pid file path (default ~/.config/eightctl/daemon.pid)"`

	b *Backend
}

func (c *daemonCmd) Run(ctx *cli.Context) error {
	req := op.Request{Surface: op.SurfaceCLI}
	s, err := c.b.load(req.Surface)
	if err != nil {
		return err
	}
	cl := c.b.newClient(s)
	if !c.DryRun {
		if cl, _, err = c.b.client(req); err != nil {
			return err
		}
	}
	if s.Path == "" {
		return usage("no config file loaded; specify --config")
	}
	var raw struct {
		Schedule []daemon.ScheduleItem `yaml:"schedule"`
	}
	if err := yaml.Unmarshal(s.Data, &raw); err != nil {
		return usage("%v", err)
	}
	if len(raw.Schedule) == 0 {
		return usage("no schedule entries found")
	}
	loc := time.Local
	if s.Timezone != "local" {
		if loc, err = time.LoadLocation(s.Timezone); err != nil {
			return usage("load timezone: %v", err)
		}
	}
	pid := c.PIDFile
	if pid == "" {
		if home, err := os.UserHomeDir(); err == nil {
			pid = filepath.Join(home, ".config", "eightctl", "daemon.pid")
		}
	}
	r := daemon.Runner{Items: raw.Schedule, Client: cl, Timezone: loc, DryRun: c.DryRun, Sync: c.SyncState, PIDFile: pid}
	return providerErr(r.Run(ctx))
}
