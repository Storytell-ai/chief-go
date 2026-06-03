package chief

import (
	"context"
	"net/http"
	"net/url"
)

// LabelsService is the project-scoped label surface. Every method needs a
// project ID configured on the parent Client.
type LabelsService struct {
	client *Client
}

// Create mints a label in the configured project.
func (s *LabelsService) Create(ctx context.Context, req *CreateLabelRequest) (*LabelSummary, error) {
	var label LabelSummary
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/labels", req, &label); err != nil {
		return nil, err
	}
	return &label, nil
}

// List returns a page of labels in the configured project.
func (s *LabelsService) List(ctx context.Context, opts ...ListOption) (*LabelPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/labels"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page LabelPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns a label's full view.
func (s *LabelsService) Get(ctx context.Context, labelID string) (*LabelResponse, error) {
	var label LabelResponse
	path := "/v1/labels/" + url.PathEscape(labelID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &label); err != nil {
		return nil, err
	}
	return &label, nil
}

// Update patches a label's display metadata. Nil pointer fields on the request
// leave the corresponding field unchanged.
func (s *LabelsService) Update(ctx context.Context, labelID string, req *UpdateLabelRequest) (*LabelResponse, error) {
	var label LabelResponse
	path := "/v1/labels/" + url.PathEscape(labelID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &label); err != nil {
		return nil, err
	}
	return &label, nil
}

// Delete removes a label.
func (s *LabelsService) Delete(ctx context.Context, labelID string) error {
	path := "/v1/labels/" + url.PathEscape(labelID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}
