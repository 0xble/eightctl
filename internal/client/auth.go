package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"log/slog"
	"github.com/0xble/eightsleep/internal/tokencache"
)

// Authenticate fetches a bearer token via the OAuth password-grant endpoint.
func (c *Client) Authenticate(ctx context.Context) error {
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", c.Email)
	form.Set("password", c.Password)
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.authEndpoint(),
		strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		slog.Debug("token auth failed", "status", resp.StatusCode)
		return &APIError{Method: http.MethodPost, Status: resp.StatusCode, Token: true,
			msg: fmt.Sprintf("token auth failed: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))}
	}

	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		UserID      string `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return err
	}
	if res.AccessToken == "" {
		return errors.New("empty access token")
	}
	c.token = res.AccessToken
	if res.ExpiresIn == 0 {
		res.ExpiresIn = 3600
	}
	c.tokenExp = time.Now().Add(time.Duration(res.ExpiresIn-60) * time.Second)
	if c.UserID == "" {
		c.UserID = res.UserID
	}
	if err := tokencache.Save(c.Identity(), c.token, c.tokenExp, c.UserID); err != nil {
		slog.Debug("failed to cache token", "error", err)
	} else {
		slog.Debug("saved token to cache", "expires_at", c.tokenExp)
	}
	return nil
}

func (c *Client) ensureToken(ctx context.Context) error {
	if c.token != "" && time.Now().Before(c.tokenExp) {
		slog.Debug("using in-memory token", "expires_in", time.Until(c.tokenExp).Round(time.Second))
		return nil
	}
	// Trust cached tokens without server validation. If token is invalid,
	// the server will return 401 and we'll clear cache + re-authenticate.
	if cached, err := tokencache.Load(c.Identity()); err == nil {
		slog.Debug("loaded token from cache", "expires_at", cached.ExpiresAt, "user_id", cached.UserID)
		c.token = cached.Token
		c.tokenExp = cached.ExpiresAt
		if cached.UserID != "" && c.UserID == "" {
			c.UserID = cached.UserID
		}
		return nil
	} else if errors.Is(err, tokencache.ErrAmbiguousAccount) {
		return err
	} else {
		slog.Debug("no cached token", "reason", err)
	}
	slog.Debug("authenticating with server")
	return c.Authenticate(ctx)
}

func (c *Client) Identity() tokencache.Identity {
	return tokencache.Identity{
		BaseURL:  c.BaseURL,
		ClientID: c.ClientID,
		Email:    c.Email,
	}
}
