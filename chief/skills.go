package chief

import (
	"context"
	"net/http"
	"net/url"
)

// SkillsService is the project-scoped skill surface. Every method needs a
// project ID configured on the parent Client.
type SkillsService struct {
	client *Client
}

// Create mints a skill in the configured project.
func (s *SkillsService) Create(ctx context.Context, req *CreateSkillRequest) (*SkillResponse, error) {
	var skill SkillResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/skills", req, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// List returns a cursor page of the skills visible in the configured project.
func (s *SkillsService) List(ctx context.Context, opts ...ListOption) (*SkillPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/skills"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page SkillPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns a skill's full view.
func (s *SkillsService) Get(ctx context.Context, skillID string) (*SkillResponse, error) {
	var skill SkillResponse
	path := "/v1/skills/" + url.PathEscape(skillID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// Update patches a skill. Nil pointer fields on the request leave the
// corresponding field unchanged.
func (s *SkillsService) Update(ctx context.Context, skillID string, req *UpdateSkillRequest) (*SkillResponse, error) {
	var skill SkillResponse
	path := "/v1/skills/" + url.PathEscape(skillID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// Delete removes a skill.
func (s *SkillsService) Delete(ctx context.Context, skillID string) error {
	path := "/v1/skills/" + url.PathEscape(skillID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// Enable turns the skill on for the caller.
func (s *SkillsService) Enable(ctx context.Context, skillID string) (*SkillResponse, error) {
	var skill SkillResponse
	path := "/v1/skills/" + url.PathEscape(skillID) + "/enable"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}

// Disable turns the skill off for the caller.
func (s *SkillsService) Disable(ctx context.Context, skillID string) (*SkillResponse, error) {
	var skill SkillResponse
	path := "/v1/skills/" + url.PathEscape(skillID) + "/disable"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &skill); err != nil {
		return nil, err
	}
	return &skill, nil
}
