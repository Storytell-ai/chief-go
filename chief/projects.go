package chief

import (
	"context"
	"net/http"
)

// ProjectsService lists the projects the API key can reach. It needs no
// project ID on the Client.
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
