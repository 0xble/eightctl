package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// TemperatureSchedule represents server-side temperature schedules.
type TemperatureSchedule struct {
	ID         string `json:"id"`
	StartTime  string `json:"startTime"`
	Level      int    `json:"level"`
	DaysOfWeek []int  `json:"daysOfWeek"`
	Enabled    bool   `json:"enabled"`
}

func (c *Client) ListSchedules(ctx context.Context) ([]TemperatureSchedule, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("specialization", "all")
	path := fmt.Sprintf("/users/%s/temperature/schedules", c.UserID)
	var res struct {
		Schedules []TemperatureSchedule `json:"schedules"`
	}
	if err := c.do(ctx, http.MethodGet, path, q, nil, &res); err == nil {
		return res.Schedules, nil
	}
	// Fallback: some accounts reject /temperature/schedules entirely; household schedule still works.
	var household struct {
		Schedules []TemperatureSchedule `json:"schedules"`
	}
	path = fmt.Sprintf("/household/users/%s/schedule", c.UserID)
	if err := c.do(ctx, http.MethodGet, path, nil, nil, &household); err != nil {
		return nil, err
	}
	return household.Schedules, nil
}

func (c *Client) CreateSchedule(ctx context.Context, s TemperatureSchedule) (*TemperatureSchedule, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/temperature/schedules", c.UserID)
	var res struct {
		Schedule TemperatureSchedule `json:"schedule"`
	}
	if err := c.do(ctx, http.MethodPost, path, nil, s, &res); err != nil {
		return nil, err
	}
	return &res.Schedule, nil
}

func (c *Client) UpdateSchedule(ctx context.Context, id string, patch map[string]any) (*TemperatureSchedule, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	path := fmt.Sprintf("/users/%s/temperature/schedules/%s", c.UserID, id)
	var res struct {
		Schedule TemperatureSchedule `json:"schedule"`
	}
	if err := c.do(ctx, http.MethodPatch, path, nil, patch, &res); err != nil {
		return nil, err
	}
	return &res.Schedule, nil
}

func (c *Client) DeleteSchedule(ctx context.Context, id string) error {
	if err := c.requireUser(ctx); err != nil {
		return err
	}
	path := fmt.Sprintf("/users/%s/temperature/schedules/%s", c.UserID, id)
	return c.do(ctx, http.MethodDelete, path, nil, nil, nil)
}
