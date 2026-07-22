package gh

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/google/go-github/v45/github"
)

// WrapGitHubError examines a GitHub API error and returns a more descriptive
// error message. GitHub often returns 404 instead of 401/403 for security
// reasons (to avoid revealing resource existence to unauthorized users), so
// a "404 Not Found" may actually mean the token is expired, revoked, or
// lacks the required permissions.
func WrapGitHubError(err error, action string) error {
	if err == nil {
		return nil
	}

	var ghErr *github.ErrorResponse
	if !errors.As(err, &ghErr) {
		return fmt.Errorf("%s: %w", action, err)
	}

	status := ghErr.Response.StatusCode
	msg := ghErr.Message

	switch status {
	case http.StatusUnauthorized: // 401
		return fmt.Errorf("%s: authentication failed (HTTP 401): %s (is the token valid/expired?)", action, msg)
	case http.StatusForbidden: // 403
		return fmt.Errorf("%s: access denied (HTTP 403): %s (is the token valid and does it have the required scopes?)", action, msg)
	case http.StatusNotFound: // 404
		return fmt.Errorf("%s: not found (HTTP 404): %s (if this resource exists, the token may be expired, revoked, or lack access — GitHub returns 404 instead of 401/403 for security)", action, msg)
	default:
		return fmt.Errorf("%s: GitHub API error (HTTP %d): %s", action, status, msg)
	}
}
