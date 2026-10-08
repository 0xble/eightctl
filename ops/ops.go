// Package ops declares every eightsleep operation once. The CLI, the HTTP API
// on a Unix socket, MCP and the metadata document are generated from these
// declarations by the toolkit.
//
// Writes change a real bed: temperature, power, alarms, away mode and audio.
// On the command line they apply immediately, as eightctl's always did, and
// --dry-run previews them. HTTP and MCP callers must send "apply": true, and
// deletes also "confirm": true.
package ops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/0xble/toolkit"
	"github.com/0xble/toolkit/cli"
	"github.com/0xble/toolkit/op"

	"github.com/0xble/eightsleep/internal/client"
	"github.com/0xble/eightsleep/internal/config"
	"github.com/0xble/eightsleep/internal/tokencache"
)

// Description is the root help text.
const Description = "Control an Eight Sleep Pod: temperature, power, alarms, away mode, audio and sleep data."

// Globals are the root flags. Timezone's json tag binds it into the timezone
// input of the operations that have one. The rest are read by the backend:
// credentials must never be operation inputs or travel in a request body.
type Globals struct {
	Config       string `help:"Config file (default ~/.config/eightsleep/config.yaml if present, else ~/.config/eightctl/config.yaml)"`
	Verbose      bool   `short:"v" help:"Verbose logging"`
	Email        string `help:"Eight Sleep account email"`
	Password     string `help:"Eight Sleep account password"`
	ClientID     string `name:"client-id" help:"Eight Sleep client ID (optional; defaults to public app client)"`
	ClientSecret string `name:"client-secret" help:"Eight Sleep client secret (optional; defaults to public app client)"`
	UserID       string `name:"user-id" help:"Eight Sleep user ID"`
	Timezone     string `json:"timezone" help:"IANA timezone (e.g., America/New_York) or 'local'"`
	Output       string `help:"Human output format: table|json|csv (default table)"`
	Quiet        bool   `help:"Suppress the config load message"`
}

// Backend is what the operations reach the outside world through. Tests
// replace the hosts, the environment, the clock and the local zone.
type Backend struct {
	Globals *Globals
	// Hosts replaces the Eight Sleep hosts when set.
	Hosts *client.Hosts
	// Getenv reads the environment. Nil means os.Getenv.
	Getenv func(string) string
	// Stderr receives notices such as the config-file line. Nil means os.Stderr.
	Stderr io.Writer
	// Now is the clock. Nil means time.Now.
	Now func() time.Time
	// LocalZone names the system's IANA zone for --timezone local. Nil
	// reads TZ and /etc/localtime.
	LocalZone func() string
	// Version is the tool version, for the version operation.
	Version string

	mu       sync.Mutex
	settings *config.Settings
}

// NewBackend is the production backend.
func NewBackend(version string) *Backend {
	return &Backend{Globals: &Globals{}, Version: version}
}

// New returns the registry with every operation registered.
func New(version string, b *Backend) *op.Registry {
	if b.Version == "" {
		b.Version = version
	}
	reg := op.New("eightsleep", version)
	Register(reg, b)
	return reg
}

// Register declares every operation on reg.
func Register(reg *op.Registry, b *Backend) {
	registerAccount(reg, b)
	registerPower(reg, b)
	registerAlarms(reg, b)
	registerAway(reg, b)
	registerSleep(reg, b)
	registerTempModes(reg, b)
	registerReads(reg, b)
	registerActions(reg, b)
}

// Options are the toolkit options: root flags and the hand-written logout
// and daemon commands, which handle the credential cache and a long-running
// schedule and so stay CLI-only.
func Options(b *Backend) toolkit.Options {
	return toolkit.Options{
		Description: Description,
		Globals:     b.Globals,
		Commands: []cli.Command{
			{Name: "logout", Help: "Clear cached authentication token", Cmd: &logoutCmd{b: b}},
			{Name: "daemon", Help: "Run schedule daemon from config file", Cmd: &daemonCmd{b: b}},
		},
	}
}

func (b *Backend) getenv(key string) string {
	if b.Getenv != nil {
		return b.Getenv(key)
	}
	return os.Getenv(key)
}

func (b *Backend) stderr() io.Writer {
	if b.Stderr != nil {
		return b.Stderr
	}
	return os.Stderr
}

func (b *Backend) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *Backend) globals() Globals {
	if b.Globals == nil {
		return Globals{}
	}
	return *b.Globals
}

// load reads the settings once per process. The command line prints the
// config-file notice and permission warning on stderr, as eightctl did.
func (b *Backend) load(surface op.Surface) (config.Settings, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.settings != nil {
		return *b.settings, nil
	}
	g := b.globals()
	home, err := os.UserHomeDir()
	if err != nil {
		return config.Settings{}, op.Errorf(op.KindError, "config_error", "find home: %v", err)
	}
	s, err := config.Load(config.Flags{Config: g.Config, Email: g.Email, Password: g.Password, UserID: g.UserID,
		ClientID: g.ClientID, ClientSecret: g.ClientSecret, Timezone: g.Timezone, Output: g.Output,
		Verbose: g.Verbose, Quiet: g.Quiet}, home, b.getenv)
	if err != nil {
		return config.Settings{}, op.Errorf(op.KindError, "config_error", "%v", err)
	}
	if surface == op.SurfaceCLI {
		if s.Path != "" && !s.Quiet {
			_, _ = fmt.Fprintf(b.stderr(), "Using config file: %s\n", s.Path)
		}
		if s.Insecure() {
			_, _ = fmt.Fprintf(b.stderr(), "warning: config file %s permissions %o; suggest 600\n", s.Path, s.Mode)
		}
	}
	if s.Verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
	}
	b.settings = &s
	return s, nil
}

// newClient builds a client from the settings, without checking credentials.
func (b *Backend) newClient(s config.Settings) *client.Client {
	cl := client.New(s.Email, s.Password, s.UserID, s.ClientID, s.ClientSecret)
	cl.Now = b.Now
	if b.Hosts != nil {
		cl.UseHosts(*b.Hosts)
	}
	return cl
}

// client returns an authenticated-capable client: a cached token or an email
// and password must be available, as eightctl's requireAuthFields demanded.
func (b *Backend) client(req op.Request) (*client.Client, config.Settings, error) {
	s, err := b.load(req.Surface)
	if err != nil {
		return nil, s, err
	}
	cl := b.newClient(s)
	cached, err := tokencache.Load(cl.Identity())
	switch {
	case err == nil:
		if cl.UserID == "" {
			cl.UserID = cached.UserID
		}
		return cl, s, nil
	case errors.Is(err, tokencache.ErrAmbiguousAccount):
		return nil, s, &op.Error{Kind: op.KindAuth, Code: "ambiguous_account", Message: err.Error(),
			Suggestions: []string{"pass --email to choose the cached account"}}
	}
	var missing []string
	if s.Email == "" {
		missing = append(missing, "email")
	}
	if s.Password == "" {
		missing = append(missing, "password")
	}
	if len(missing) > 0 {
		return nil, s, &op.Error{Kind: op.KindAuth, Code: "auth_required",
			Message:     "missing required auth fields: " + strings.Join(missing, ", "),
			Suggestions: []string{"set email and password in ~/.config/eightctl/config.yaml, EIGHTCTL_EMAIL and EIGHTCTL_PASSWORD, or --email and --password"}}
	}
	return cl, s, nil
}

// usage is a validation failure of the caller's input.
func usage(format string, args ...any) *op.Error {
	return op.Errorf(op.KindUsage, "usage", format, args...)
}

// providerErr classifies a client failure into the family error kinds. The
// message is eightctl's.
func providerErr(err error) error {
	if err == nil {
		return nil
	}
	var oe *op.Error
	if errors.As(err, &oe) {
		return oe
	}
	var ae *client.APIError
	if errors.As(err, &ae) {
		e := &op.Error{Kind: op.KindError, Code: "provider_error", Message: err.Error(), HTTPStatus: ae.Status}
		switch {
		case ae.Token || ae.Status == http.StatusUnauthorized:
			e.Kind, e.Code = op.KindAuth, "auth_failed"
		case ae.Status == http.StatusForbidden:
			e.Kind, e.Code = op.KindAuth, "forbidden"
		case ae.Status == http.StatusNotFound:
			e.Kind, e.Code = op.KindNotFound, "not_found"
		case ae.Status == http.StatusTooManyRequests:
			e.Kind, e.Code = op.KindRate, "rate_limited"
		}
		return e
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &op.Error{Kind: op.KindTimeout, Code: "timeout", Message: err.Error()}
	}
	if errors.Is(err, tokencache.ErrAmbiguousAccount) {
		return &op.Error{Kind: op.KindAuth, Code: "ambiguous_account", Message: err.Error()}
	}
	return op.Errorf(op.KindError, "error", "%s", err.Error())
}
