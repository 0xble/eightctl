package client

import (
	"context"
	"fmt"
	"net/http"
)

type BaseActions struct{ c *Client }

func (c *Client) Base() *BaseActions { return &BaseActions{c: c} }

func (b *BaseActions) Info(ctx context.Context) (any, error) {
	if err := b.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/base", b.c.UserID)
	var res any
	err := b.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (b *BaseActions) SetAngle(ctx context.Context, head, foot int) error {
	if err := b.c.requireUser(ctx); err != nil {
		return err
	}
	deviceID, err := b.c.EnsureDeviceID(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/base/angle", b.c.UserID)
	body := map[string]any{
		"torsoAngle":   head,
		"legAngle":     foot,
		"deviceId":     deviceID,
		"deviceOnline": true,
	}
	return b.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (b *BaseActions) Presets(ctx context.Context) (any, error) {
	if err := b.c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/base/presets", b.c.UserID)
	var res any
	err := b.c.do(ctx, http.MethodGet, path, nil, nil, &res)
	return res, err
}

func (b *BaseActions) RunPreset(ctx context.Context, name string) error {
	if err := b.c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/base/presets", b.c.UserID)
	body := map[string]any{"name": name}
	return b.c.do(ctx, http.MethodPost, path, nil, body, nil)
}

func (b *BaseActions) VibrationTest(ctx context.Context) error {
	deviceID, err := b.c.EnsureDeviceID(ctx)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/devices/%s/vibration-test", deviceID)
	return b.c.do(ctx, http.MethodPost, path, nil, map[string]any{}, nil)
}
