package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type TempModes struct{ c *Client }

func (c *Client) TempModes() *TempModes { return &TempModes{c: c} }

// Nap mode controls
func (t *TempModes) NapActivate(ctx context.Context) error {
	return t.simplePost(ctx, "/temperature/nap-mode/activate")
}

func (t *TempModes) NapDeactivate(ctx context.Context) error {
	return t.simplePost(ctx, "/temperature/nap-mode/deactivate")
}

func (t *TempModes) NapExtend(ctx context.Context) error {
	return t.simplePost(ctx, "/temperature/nap-mode/extend")
}

func (t *TempModes) NapStatus(ctx context.Context, out any) error {
	err := t.simpleGet(ctx, "/temperature/nap-mode/status", out)
	if err == nil {
		return nil
	}

	// Current app-api returns 404 "No active nap session found" for status when idle.
	if strings.Contains(strings.ToLower(err.Error()), "no active nap session found") {
		var cfg map[string]any
		if cfgErr := t.simpleGet(ctx, "/temperature/nap-mode", &cfg); cfgErr == nil {
			if m, ok := out.(*map[string]any); ok && m != nil {
				*m = map[string]any{
					"active": false,
					"config": cfg,
				}
				return nil
			}
		}
	}
	return err
}

// Hot flash controls
func (t *TempModes) HotFlashActivate(ctx context.Context) error {
	return t.simplePost(ctx, "/temperature/hot-flash-mode/activate")
}

func (t *TempModes) HotFlashDeactivate(ctx context.Context) error {
	return t.simplePost(ctx, "/temperature/hot-flash-mode/deactivate")
}

func (t *TempModes) HotFlashStatus(ctx context.Context, out any) error {
	return t.simpleGet(ctx, "/temperature/hot-flash-mode", out)
}

// Temp events history
func (t *TempModes) TempEvents(ctx context.Context, from, to string, out any) error {
	if err := t.c.requireUser(ctx); err != nil {
		return err
	}
	q := url.Values{}
	if from != "" {
		q.Set("from", from)
	}
	if to != "" {
		q.Set("to", to)
	}
	path := fmt.Sprintf("/users/%s/temp-events", t.c.UserID)
	return t.c.do(ctx, http.MethodGet, path, q, nil, out)
}

func (t *TempModes) simplePost(ctx context.Context, suffix string) error {
	if err := t.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s%s", t.c.UserID, suffix)
	return t.c.do(ctx, http.MethodPost, path, nil, map[string]string{}, nil)
}

func (t *TempModes) simpleGet(ctx context.Context, suffix string, out any) error {
	if err := t.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s%s", t.c.UserID, suffix)
	return t.c.do(ctx, http.MethodGet, path, nil, nil, out)
}
