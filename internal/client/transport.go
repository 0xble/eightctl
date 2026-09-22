package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"charm.land/log/v2"
	"github.com/steipete/eightctl/internal/tokencache"
)

const maxRetries = 3

// do builds a URL from BaseURL + path and delegates to doBase. Client-API
// routes may fall back to the sibling API host when the provider reports the
// route as unavailable on the primary base (EIGHTCTL-002 API-drift tolerance).
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doBase(ctx, c.BaseURL, method, path, query, body, out, true)
}

// doApp routes to AppURL without cross-base fallback (app-api has no sibling host).
func (c *Client) doApp(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	return c.doBase(ctx, c.AppURL, method, path, query, body, out, false)
}

// doURL sends an authenticated request to an absolute URL. Use do() for
// BaseURL-relative paths; use doURL directly for requests to other hosts
// (e.g. the app API for away mode).
func (c *Client) doURL(ctx context.Context, method, u string, body any, out any) error {
	_, err := c.doRequest(ctx, method, u, body, out, false)
	return err
}

// doBase resolves base+path+query and applies cross-base fallback once when the
// provider reports the route missing on the primary base.
func (c *Client) doBase(ctx context.Context, baseURL, method, path string, query url.Values, body any, out any, allowFallback bool) error {
	u := baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	retryBase, err := c.doRequest(ctx, method, u, body, out, allowFallback)
	if err == nil || !retryBase {
		return err
	}
	alt := alternateBase(baseURL)
	if alt == "" {
		return err
	}
	log.Debug("retrying on alternate API base", "from", baseURL, "to", alt, "method", method, "path", path)
	altURL := alt + path
	if len(query) > 0 {
		altURL += "?" + query.Encode()
	}
	_, altErr := c.doRequest(ctx, method, altURL, body, out, false)
	return altErr
}

// doRequest performs the authenticated request with bounded, cancellable
// 401/429 retries. It reports whether the caller should retry on the
// alternate API base instead of surfacing the error.
func (c *Client) doRequest(ctx context.Context, method, u string, body any, out any, allowFallback bool) (retryAlternateBase bool, err error) {
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if err := c.ensureToken(ctx); err != nil {
			return false, err
		}
		var rdr io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return false, err
			}
			rdr = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, u, rdr)
		if err != nil {
			return false, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Connection", "keep-alive")
		req.Header.Set("User-Agent", "okhttp/4.9.3")
		// Leave Accept-Encoding to Go so gzip responses are decoded transparently.
		resp, err := c.HTTP.Do(req)
		if err != nil {
			return false, err
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests:
			resp.Body.Close()
			if attempt >= maxRetries {
				return false, fmt.Errorf("rate limited after %d retries: %s %s", maxRetries, method, u)
			}
			timer := time.NewTimer(time.Duration(2*(attempt+1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return false, ctx.Err()
			case <-timer.C:
			}
		case http.StatusUnauthorized:
			resp.Body.Close()
			if attempt >= maxRetries {
				return false, fmt.Errorf("unauthorized after %d retries: %s %s", maxRetries, method, u)
			}
			c.token = ""
			_ = tokencache.Clear(c.Identity())
			// A failed cache removal must not reload the token the server just rejected.
			if err := c.Authenticate(ctx); err != nil {
				return false, err
			}
		default:
			defer resp.Body.Close()
			reader, err := decodedBody(resp)
			if err != nil {
				return false, err
			}
			if resp.StatusCode >= 300 {
				b, _ := io.ReadAll(reader)
				if allowFallback && shouldTryFallbackBase(u, resp.StatusCode, b) {
					return true, fmt.Errorf("api %s %s: status %d: %s", method, u, resp.StatusCode, string(b))
				}
				return false, fmt.Errorf("api %s %s: status %d: %s", method, u, resp.StatusCode, string(b))
			}
			if out != nil {
				return false, json.NewDecoder(reader).Decode(out)
			}
			return false, nil
		}
	}
}

// decodedBody transparently decompresses gzip responses. Go decodes gzip
// automatically only when it set Accept-Encoding itself, so providers that
// compress unconditionally still need explicit handling.
func decodedBody(resp *http.Response) (io.Reader, error) {
	if resp.Header.Get("Content-Encoding") != "gzip" {
		return resp.Body, nil
	}
	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return nil, err
	}
	return gr, nil
}

func alternateBase(baseURL string) string {
	switch baseURL {
	case defaultBaseURL:
		return fallbackBaseURL
	case fallbackBaseURL:
		return defaultBaseURL
	default:
		return ""
	}
}

// shouldTryFallbackBase reports whether a failed request looks like the route
// simply does not exist on this API host, rather than a real error state.
func shouldTryFallbackBase(requestURL string, statusCode int, body []byte) bool {
	if bytes.HasPrefix([]byte(requestURL), []byte(fallbackBaseURL)) {
		return false
	}
	lower := bytes.ToLower(body)
	// Some app-api endpoints use 404 for valid "inactive" states; do not reroute those.
	if bytes.Contains(lower, []byte("no active nap session found")) {
		return false
	}
	if statusCode == http.StatusNotFound {
		return true
	}
	return bytes.Contains(lower, []byte("cannot get /v1/"))
}
