package chief

import (
	"context"
	"net/http"
)

// SearchService is the project-scoped knowledge base search surface. It needs a
// project ID configured on the parent Client, and the organization's plan must
// include search — a free plan is refused with 403.
type SearchService struct {
	client *Client
}

// Search runs a semantic search over the ingested assets in the configured
// project and returns the matching passages, best first.
//
// A nil req.Scope searches the whole project. Passing a non-nil Scope whose
// lists are all empty searches nothing and returns no results, which is how a
// caller states it wants no project knowledge rather than all of it.
//
// Asset ids the configured project cannot reach are dropped rather than
// rejected: they contribute nothing and the response does not reveal that they
// exist.
func (s *SearchService) Search(ctx context.Context, req *SearchRequest) (*SearchResponse, error) {
	var resp SearchResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/search", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
