package chief

import (
	"context"
	"net/http"
	"net/url"
)

// ActionsService is the project-scoped action surface. Every method needs a
// project ID configured on the parent Client.
type ActionsService struct {
	client *Client
}

// Create mints an action in the configured project.
func (s *ActionsService) Create(ctx context.Context, req *ActionRequest) (*ActionResponse, error) {
	var action ActionResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/actions", req, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// List returns a page of actions in the configured project.
func (s *ActionsService) List(ctx context.Context, opts ...ListOption) (*ActionPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/actions"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page ActionPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns an action's current state.
func (s *ActionsService) Get(ctx context.Context, actionID string) (*ActionResponse, error) {
	var action ActionResponse
	path := "/v1/actions/" + url.PathEscape(actionID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// Update applies a patch to an action. Nil pointer fields on the request leave
// the corresponding section unchanged.
func (s *ActionsService) Update(ctx context.Context, actionID string, req *ActionRequest) (*ActionResponse, error) {
	var action ActionResponse
	path := "/v1/actions/" + url.PathEscape(actionID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// Delete removes an action.
func (s *ActionsService) Delete(ctx context.Context, actionID string) error {
	path := "/v1/actions/" + url.PathEscape(actionID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// Enable activates an action so it runs on its schedule or trigger.
func (s *ActionsService) Enable(ctx context.Context, actionID string) (*ActionResponse, error) {
	var action ActionResponse
	path := "/v1/actions/" + url.PathEscape(actionID) + "/enable"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &action); err != nil {
		return nil, err
	}
	return &action, nil
}

// Disable pauses an action without deleting it.
func (s *ActionsService) Disable(ctx context.Context, actionID string) (*ActionResponse, error) {
	var action ActionResponse
	path := "/v1/actions/" + url.PathEscape(actionID) + "/disable"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &action); err != nil {
		return nil, err
	}
	return &action, nil
}
