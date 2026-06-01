package chief

import (
	"errors"
	"fmt"
	"net/http"
)

// APIError is the decoded error envelope every non-2xx response carries. Code
// is a stable machine-readable identifier; Humane is the user-facing message;
// StatusCode mirrors the HTTP status.
type APIError struct {
	Code       string `json:"code"`
	Humane     string `json:"humane"`
	StatusCode int    `json:"statusCode"`
}

func (e *APIError) Error() string {
	switch {
	case e.Code != "" && e.Humane != "":
		return fmt.Sprintf("chief: %s (%s, status %d)", e.Humane, e.Code, e.StatusCode)
	case e.Humane != "":
		return fmt.Sprintf("chief: %s (status %d)", e.Humane, e.StatusCode)
	case e.Code != "":
		return fmt.Sprintf("chief: %s (status %d)", e.Code, e.StatusCode)
	default:
		return fmt.Sprintf("chief: request failed with status %d", e.StatusCode)
	}
}

// IsAPIError extracts the *APIError from an error chain.
func IsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}

// IsNotFound reports whether err is a 404 from the API.
func IsNotFound(err error) bool { return hasStatus(err, http.StatusNotFound) }

// IsUnauthorized reports whether err is a 401 from the API.
func IsUnauthorized(err error) bool { return hasStatus(err, http.StatusUnauthorized) }

// IsRateLimited reports whether err is a 429 from the API.
func IsRateLimited(err error) bool { return hasStatus(err, http.StatusTooManyRequests) }

// IsBadRequest reports whether err is a 400 from the API.
func IsBadRequest(err error) bool { return hasStatus(err, http.StatusBadRequest) }

func hasStatus(err error, status int) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == status
}
