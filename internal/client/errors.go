package client

import "strings"

// IsEndpointUnavailable returns true when the API indicates the requested route
// does not exist for the current backend/account.
func IsEndpointUnavailable(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "cannot get") ||
		strings.Contains(s, "cannot post") ||
		strings.Contains(s, "cannot put") ||
		strings.Contains(s, "cannot patch") ||
		strings.Contains(s, "cannot delete")
}
