package client

import (
	"context"
	"fmt"
	"net/http"
)

// Alarm represents alarm payload.
type Alarm struct {
	ID         string  `json:"id"`
	Enabled    bool    `json:"enabled"`
	Time       string  `json:"time"`
	DaysOfWeek []int   `json:"daysOfWeek"`
	Vibration  any     `json:"vibration"`
	Sound      *string `json:"sound,omitempty"`
}

func (c *Client) ListAlarms(ctx context.Context) ([]Alarm, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/alarms", c.UserID)
	var res struct {
		Alarms []Alarm `json:"alarms"`
	}
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &res); err != nil {
		return nil, err
	}
	return res.Alarms, nil
}

func (c *Client) CreateAlarm(ctx context.Context, alarm Alarm) (*Alarm, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/alarms", c.UserID)
	var res struct {
		Alarm Alarm `json:"alarm"`
	}
	if err := c.do(ctx, http.MethodPost, path, nil, alarm, &res); err != nil {
		return nil, err
	}
	return &res.Alarm, nil
}

func (c *Client) UpdateAlarm(ctx context.Context, alarmID string, patch map[string]any) (*Alarm, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/alarms/%s", c.UserID, alarmID)
	var res struct {
		Alarm Alarm `json:"alarm"`
	}
	if err := c.do(ctx, http.MethodPatch, path, nil, patch, &res); err != nil {
		return nil, err
	}
	return &res.Alarm, nil
}

func (c *Client) DeleteAlarm(ctx context.Context, alarmID string) error {
	if err := c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/alarms/%s", c.UserID, alarmID)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
