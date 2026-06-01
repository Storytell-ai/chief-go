package chief

import (
	"context"
	"net/http"
	"net/url"
)

// SessionsService is the project-scoped session surface. Every method needs a
// project ID configured on the parent Client.
type SessionsService struct {
	client *Client
}

// List returns a cursor page of the caller's sessions in the configured
// project.
func (s *SessionsService) List(ctx context.Context, opts ...ListOption) (*SessionPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/sessions"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var list SessionPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// Get returns a session's full view, including its transcript.
func (s *SessionsService) Get(ctx context.Context, sessionID string) (*SessionResponse, error) {
	var session SessionResponse
	path := "/v1/sessions/" + url.PathEscape(sessionID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Update patches a session's name and/or description. Nil pointer fields on the
// request leave the corresponding field unchanged.
func (s *SessionsService) Update(ctx context.Context, sessionID string, req *UpdateSessionRequest) (*SessionResponse, error) {
	var session SessionResponse
	path := "/v1/sessions/" + url.PathEscape(sessionID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

// Delete removes a session.
func (s *SessionsService) Delete(ctx context.Context, sessionID string) error {
	path := "/v1/sessions/" + url.PathEscape(sessionID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}
