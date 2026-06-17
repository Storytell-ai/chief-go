package chief

import (
	"context"
	"net/http"
	"net/url"
)

// ProjectsService manages projects, their members, and invitations. Its routes
// are authenticated but not project-scoped, so no project ID is needed on the
// Client.
type ProjectsService struct {
	client *Client
}

// List returns the projects the API key can access.
func (s *ProjectsService) List(ctx context.Context) (*ProjectPage, error) {
	var page ProjectPage
	if _, err := s.client.Do(ctx, http.MethodGet, "/v1/projects", nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Create mints a project in the org and workspace of the caller's root grant.
func (s *ProjectsService) Create(ctx context.Context, req *CreateProjectRequest) (*Project, error) {
	var project Project
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/projects", req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// Update replaces a project's name and description — the only two mutable
// fields. The body is a full set, not a patch: an empty Description clears it.
func (s *ProjectsService) Update(ctx context.Context, projectID string, req *UpdateProjectRequest) (*Project, error) {
	var project Project
	path := "/v1/projects/" + url.PathEscape(projectID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &project); err != nil {
		return nil, err
	}
	return &project, nil
}

// ListMembers returns every user holding a grant in the project.
func (s *ProjectsService) ListMembers(ctx context.Context, projectID string) (*ProjectMemberList, error) {
	var list ProjectMemberList
	path := "/v1/projects/" + url.PathEscape(projectID) + "/members"
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

// CreateInvitation invites one user to the project by email. The invitation
// email is sent automatically; the user becomes a member only after accepting.
// A project holds one invitation per role: when one already exists the email
// is added to it, and re-inviting an email already on it succeeds unchanged.
func (s *ProjectsService) CreateInvitation(ctx context.Context, projectID string, req *CreateProjectInvitationRequest) (*ProjectInvitationResponse, error) {
	var invitation ProjectInvitationResponse
	path := "/v1/projects/" + url.PathEscape(projectID) + "/invitations"
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &invitation); err != nil {
		return nil, err
	}
	return &invitation, nil
}

// DeleteInvitation revokes a pending invitation.
func (s *ProjectsService) DeleteInvitation(ctx context.Context, projectID, invitationID string) error {
	path := "/v1/projects/" + url.PathEscape(projectID) + "/invitations/" + url.PathEscape(invitationID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}
