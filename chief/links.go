package chief

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// LinksService resolves links copied from the Chief app. Its route is
// authenticated but not project-scoped, so no project ID is needed on the
// Client.
type LinksService struct {
	client *Client
}

// Resolve says what a link leads the caller to: a capture's share link or a
// session's page. A link that leads them nowhere, whatever the cause, fails
// with a 404. The debug log leaves the call out: a share link is a secret.
func (s *LinksService) Resolve(ctx context.Context, link string) (*LinkResponse, error) {
	req, err := s.client.newRequest(ctx, http.MethodPost, "/v1/links/resolve", &ResolveLinkRequest{URL: link})
	if err != nil {
		return nil, err
	}
	_, body, err := s.client.send(req, false)
	if err != nil {
		return nil, err
	}
	var resolved LinkResponse
	if err := json.Unmarshal(body, &resolved); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resolved, nil
}
