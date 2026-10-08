package client

import (
	"fmt"
	"strings"
)

// IsEndpointUnavailable returns true when the API indicates the requested route
// does not exist for the current backend/account.
func IsEndpointUnavailable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "status 404") ||
		strings.Contains(s, "cannot get") ||
		strings.Contains(s, "cannot post") ||
		strings.Contains(s, "cannot put") ||
		strings.Contains(s, "cannot patch") ||
		strings.Contains(s, "cannot delete")
}

// APIError is a failed Eight Sleep response: a non-2xx status from the API,
// retries exhausted on 401 or 429, or a refused token request. Its message is
// the one eightctl always printed.
type APIError struct {
	Method string
	URL    string
	Status int
	Body   string
	// Token marks a failure of the token request itself.
	Token bool
	msg   string
}

func (e *APIError) Error() string {
	if e.msg != "" {
		return e.msg
	}
	return fmt.Sprintf("api %s %s: status %d: %s", e.Method, e.URL, e.Status, e.Body)
}
