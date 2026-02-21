package client

import (
	"context"
	"fmt"
	"net/http"
)

type AutopilotActions struct{ c *Client }

func (c *Client) Autopilot() *AutopilotActions { return &AutopilotActions{c: c} }

func (a *AutopilotActions) Details(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/autopilotDetails", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AutopilotActions) History(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/autopilot-history", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AutopilotActions) Recap(ctx context.Context) (any, error) {
	if err := a.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/autopilotDetails/autopilotRecap", a.c.UserID)
	var res any
	err := a.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (a *AutopilotActions) SetLevelSuggestions(ctx context.Context, enabled bool) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/level-suggestions-mode", a.c.UserID)
	body := map[string]any{"enabled": enabled}
	return a.c.do(ctx, http.MethodPut, path, nil, body, nil)
}

func (a *AutopilotActions) SetSnoreMitigation(ctx context.Context, enabled bool) error {
	if err := a.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/autopilotDetails/snoringMitigation", a.c.UserID)
	body := map[string]any{
		"snoringMitigation": map[string]any{
			"enabled": enabled,
		},
	}
	return a.c.do(ctx, http.MethodPut, path, nil, body, nil)
}
