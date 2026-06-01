package chief

import (
	"context"
	"net/http"
	"net/url"
)

// MemoriesService is the project-scoped memory surface. Every method needs a
// project ID configured on the parent Client.
type MemoriesService struct {
	client *Client
}

// Create stores a memory in the configured project.
func (s *MemoriesService) Create(ctx context.Context, req *CreateMemoryRequest) (*MemoryResponse, error) {
	var memory MemoryResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/memories", req, &memory); err != nil {
		return nil, err
	}
	return &memory, nil
}

// List returns a cursor page of the memories in the configured project.
func (s *MemoriesService) List(ctx context.Context, opts ...ListOption) (*MemoryPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/memories"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page MemoryPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns a memory's full view.
func (s *MemoriesService) Get(ctx context.Context, memoryID string) (*MemoryResponse, error) {
	var memory MemoryResponse
	path := "/v1/memories/" + url.PathEscape(memoryID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &memory); err != nil {
		return nil, err
	}
	return &memory, nil
}

// Update replaces a memory's content and optionally patches its category and
// importance.
func (s *MemoriesService) Update(ctx context.Context, memoryID string, req *UpdateMemoryRequest) (*MemoryResponse, error) {
	var memory MemoryResponse
	path := "/v1/memories/" + url.PathEscape(memoryID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &memory); err != nil {
		return nil, err
	}
	return &memory, nil
}

// Delete removes a memory.
func (s *MemoriesService) Delete(ctx context.Context, memoryID string) error {
	path := "/v1/memories/" + url.PathEscape(memoryID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}
