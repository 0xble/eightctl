package client

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"time"
)

const (
	defaultBaseURL = "https://client-api.8slp.net/v1"
	// fallbackBaseURL is the sibling API host tried once when the primary base
	// reports a route as missing (EIGHTCTL-002 API-drift tolerance).
	fallbackBaseURL = "https://app-api.8slp.net/v1"
	defaultAppURL   = "https://app-api.8slp.net"
	// Extracted from the official Eight Sleep Android app v7.39.17 (public client creds)
	defaultClientID     = "0894c7f33bb94800a03f1f4df13a4f38"
	defaultClientSecret = "f0954a3ed5763ba3d06834c73731a32f15f168f47d4f164751275def86db0c76"
)

// authURL and appAPIBaseURL are vars so tests can point them at local servers.
var (
	authURL       = "https://auth-api.8slp.net/v1/tokens"
	appAPIBaseURL = "https://app-api.8slp.net/v1"
)

// Client represents Eight Sleep API client.
type Client struct {
	Email        string
	Password     string
	UserID       string
	ClientID     string
	ClientSecret string
	DeviceID     string

	HTTP     *http.Client
	BaseURL  string
	AppURL   string
	token    string
	tokenExp time.Time

	// Now is the clock for presence windows. Nil means time.Now.
	Now func() time.Time

	// hosts, when set by UseHosts, replaces the production hosts.
	hosts *Hosts
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

// Hosts replaces the three Eight Sleep hosts, each a scheme and host with no
// path, so tests and fakes never reach the real service.
type Hosts struct {
	Client string // https://client-api.8slp.net
	App    string // https://app-api.8slp.net
	Auth   string // https://auth-api.8slp.net
}

// UseHosts points every request at h. The client API keeps its fallback to
// the app API when a route is missing.
func (c *Client) UseHosts(h Hosts) {
	c.hosts = &h
	c.BaseURL = h.Client + "/v1"
	c.AppURL = h.App
}

func (c *Client) authEndpoint() string {
	if c.hosts != nil {
		return c.hosts.Auth + "/v1/tokens"
	}
	return authURL
}

func (c *Client) appAPIBase() string {
	if c.hosts != nil {
		return c.hosts.App + "/v1"
	}
	return appAPIBaseURL
}

// bases are the client API base and its sibling fallback.
func (c *Client) bases() (primary, fallback string) {
	if c.hosts != nil {
		return c.hosts.Client + "/v1", c.hosts.App + "/v1"
	}
	return defaultBaseURL, fallbackBaseURL
}

// New creates a Client.
func New(email, password, userID, clientID, clientSecret string) *Client {
	if clientID == "" {
		clientID = defaultClientID
	}
	if clientSecret == "" {
		clientSecret = defaultClientSecret
	}
	tr := &http.Transport{
		Proxy:           http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		// Disable HTTP/2; Eight Sleep frontends sometimes hang on H2 with Go.
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
	}
	return &Client{
		Email:        email,
		Password:     password,
		UserID:       userID,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTP:         &http.Client{Timeout: 20 * time.Second, Transport: tr},
		BaseURL:      defaultBaseURL,
		AppURL:       defaultAppURL,
	}
}

// EnsureUserID populates UserID by calling /users/me if missing.
func (c *Client) EnsureUserID(ctx context.Context) error {
	if c.UserID != "" {
		return nil
	}
	var res struct {
		User struct {
			UserID string `json:"userId"`
		} `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/users/me", nil, nil, &res); err != nil {
		return err
	}
	if res.User.UserID == "" {
		return errors.New("userId not found")
	}
	c.UserID = res.User.UserID
	return nil
}

// EnsureDeviceID fetches current device id if not already set.
func (c *Client) EnsureDeviceID(ctx context.Context) (string, error) {
	if c.DeviceID != "" {
		return c.DeviceID, nil
	}
	var res struct {
		User struct {
			Devices       []string `json:"devices"`
			CurrentDevice struct {
				ID string `json:"id"`
			} `json:"currentDevice"`
		} `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/users/me", nil, nil, &res); err != nil {
		return "", err
	}
	if res.User.CurrentDevice.ID != "" {
		c.DeviceID = res.User.CurrentDevice.ID
		return c.DeviceID, nil
	}
	if len(res.User.Devices) == 0 {
		return "", errors.New("no current device id")
	}
	c.DeviceID = res.User.Devices[0]
	return c.DeviceID, nil
}

func (c *Client) requireUser(ctx context.Context) error {
	return c.EnsureUserID(ctx)
}

// TokenExpiry is the expiry of the token the client holds, or zero before it
// has loaded or requested one.
func (c *Client) TokenExpiry() time.Time {
	if c.token == "" {
		return time.Time{}
	}
	return c.tokenExp
}
