package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Alarm represents alarm payload.
type Alarm struct {
	ID         string         `json:"id"`
	Enabled    bool           `json:"enabled"`
	Time       string         `json:"time"`
	DaysOfWeek []int          `json:"daysOfWeek"`
	Vibration  AlarmVibration `json:"vibration"`
	Sound      *string        `json:"sound,omitempty"`
}

// AlarmVibration supports both historical boolean payloads and current object payloads.
type AlarmVibration struct {
	Enabled    bool   `json:"enabled"`
	Pattern    string `json:"pattern,omitempty"`
	PowerLevel int    `json:"powerLevel,omitempty"`
}

func (v *AlarmVibration) UnmarshalJSON(data []byte) error {
	d := bytes.TrimSpace(data)
	if len(d) == 0 || bytes.Equal(d, []byte("null")) {
		*v = AlarmVibration{}
		return nil
	}

	var enabled bool
	if err := json.Unmarshal(d, &enabled); err == nil {
		*v = AlarmVibration{Enabled: enabled}
		return nil
	}

	var obj struct {
		Enabled    *bool  `json:"enabled"`
		Pattern    string `json:"pattern"`
		PowerLevel *int   `json:"powerLevel"`
	}
	if err := json.Unmarshal(d, &obj); err != nil {
		return err
	}

	out := AlarmVibration{Pattern: obj.Pattern}
	if obj.Enabled != nil {
		out.Enabled = *obj.Enabled
	}
	if obj.PowerLevel != nil {
		out.PowerLevel = *obj.PowerLevel
	}
	*v = out
	return nil
}

func (v AlarmVibration) MarshalJSON() ([]byte, error) {
	// Keep create/update compatibility when only a boolean intent is present.
	if v.Pattern == "" && v.PowerLevel == 0 {
		return json.Marshal(v.Enabled)
	}
	type alias AlarmVibration
	return json.Marshal(alias(v))
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
