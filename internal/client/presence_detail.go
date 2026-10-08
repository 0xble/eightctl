package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// CurrentDevice returns the user's current device record, or an empty map.
func (c *Client) CurrentDevice(ctx context.Context) (map[string]any, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	var res any
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/users/%s/current-device", c.UserID), nil, nil, &res); err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	return m, nil
}

// DeviceStatus returns the device's sensor info, online state, last contact
// and side assignments, or an empty map.
func (c *Client) DeviceStatus(ctx context.Context, deviceID string) (map[string]any, error) {
	var res any
	q := url.Values{"filter": {"sensorInfo,online,lastHeard,leftUserId,rightUserId"}}
	if err := c.do(ctx, http.MethodGet, "/devices/"+deviceID, q, nil, &res); err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	r, _ := m["result"].(map[string]any)
	return r, nil
}

// TrendDays returns the raw trend days between two dates, every session
// included.
func (c *Client) TrendDays(ctx context.Context, from, to, tz string) ([]any, error) {
	if err := c.requireUser(ctx); err != nil {
		return nil, err
	}
	q := url.Values{"tz": {tz}, "from": {from}, "to": {to}, "include-main": {"false"},
		"include-all-sessions": {"true"}, "model-version": {"v2"}}
	var res any
	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/users/%s/trends", c.UserID), q, nil, &res); err != nil {
		return nil, err
	}
	m, _ := res.(map[string]any)
	days, _ := m["days"].([]any)
	return days, nil
}
