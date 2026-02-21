package client

import (
	"context"
	"fmt"
	"net/http"
)

type HouseholdActions struct{ c *Client }

func (c *Client) Household() *HouseholdActions { return &HouseholdActions{c: c} }

func (h *HouseholdActions) Summary(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/summary", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) Schedule(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/schedule", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) CurrentSet(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/current-set", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) Invitations(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/invitations", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) Devices(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/devices", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) Users(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/users", h.c.UserID)
	var res any
	err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (h *HouseholdActions) Guests(ctx context.Context) (any, error) {
	if err := h.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/household/users/%s/guests", h.c.UserID)
	var res any
	if err := h.c.do(ctx, http.MethodGet, path, nil, nil, &res); err == nil {
		return res, nil
	} else if !IsEndpointUnavailable(err) {
		return nil, err
	}

	// Current API no longer exposes guests for some accounts; keep command usable.
	return map[string]any{"guests": []any{}}, nil
}
