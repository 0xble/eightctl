package ops_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/0xble/toolkit"
	"github.com/0xble/toolkit/op"
	"github.com/0xble/toolkit/toolkittest"
)

func httpCall(t *testing.T, f *fixture, auth op.Authorizer, name string, in map[string]any) (int, string) {
	t.Helper()
	b, _ := json.Marshal(in)
	w := httptest.NewRecorder()
	toolkit.Handler(f.reg, auth).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/ops/"+name, bytes.NewReader(b)))
	return w.Code, w.Body.String()
}

func mcpCall(t *testing.T, f *fixture, auth op.Authorizer, name string, in map[string]any) (bool, string) {
	t.Helper()
	res, err := toolkittest.MCPClient(t, f.reg, auth).CallTool(context.Background(),
		&sdk.CallToolParams{Name: strings.ReplaceAll(name, ".", "_"), Arguments: in})
	if err != nil {
		t.Fatal(err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*sdk.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return res.IsError, text.String()
}

func with(in map[string]any, kv ...any) map[string]any {
	m := map[string]any{}
	for k, v := range in {
		m[k] = v
	}
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i].(string)] = kv[i+1]
	}
	return m
}

// TestWritesRefusedWithoutApply checks every write and delete on every
// surface: nothing reaches the bed without apply, the deletes also need
// confirm off the CLI, and a served tool refuses applied writes by default.
// The CLI applies immediately, as eightctl did, and previews with --dry-run.
func TestWritesRefusedWithoutApply(t *testing.T) {
	cases := conformanceCases(t)
	for _, e := range newFixture(t).reg.Entries() {
		if !e.Effect.Mutates() {
			continue
		}
		c := cases[e.Name]
		t.Run(e.Name, func(t *testing.T) {
			changed := hasWrite

			f := newFixture(t)
			if status, body := httpCall(t, f, nil, e.Name, c.Input); status != http.StatusOK || changed(f) {
				t.Errorf("http preview: status %d, changed %v: %s", status, changed(f), body)
			}
			if isErr, text := mcpCall(t, f, op.AllowAll, e.Name, c.Input); isErr || changed(f) {
				t.Errorf("mcp preview: error %v, changed %v: %s", isErr, changed(f), text)
			}
			if code, _, stderr := f.run(t, append(c.Args, "--dry-run")...); code != 0 || changed(f) {
				t.Errorf("cli --dry-run: exit %d, changed %v: %s", code, changed(f), stderr)
			}

			applied := with(c.Input, "apply", true)
			if e.Effect == op.Destructive {
				if status, body := httpCall(t, f, op.AllowAll, e.Name, applied); status != http.StatusBadRequest ||
					toolkittest.ErrorCode([]byte(body)) != "confirmation_required" || changed(f) {
					t.Errorf("http apply without confirm: status %d: %s", status, body)
				}
				if isErr, text := mcpCall(t, f, op.AllowAll, e.Name, applied); !isErr || !strings.Contains(text, "confirmation_required") || changed(f) {
					t.Errorf("mcp apply without confirm: error %v: %s", isErr, text)
				}
				applied = with(applied, "confirm", true)
			}
			if status, body := httpCall(t, f, nil, e.Name, applied); status != http.StatusForbidden ||
				toolkittest.ErrorCode([]byte(body)) != "write_not_authorized" || changed(f) {
				t.Errorf("served apply: status %d: %s", status, body)
			}

			if code, _, stderr := f.run(t, c.Args...); code != 0 || !changed(f) {
				t.Errorf("cli immediate write: exit %d, changed %v: %s", code, changed(f), stderr)
			}
		})
	}
}

func hasWrite(f *fixture) bool {
	for _, r := range f.fake.Recorded() {
		if r.Method != http.MethodGet && r.Host != "auth-api" {
			return true
		}
	}
	return false
}
