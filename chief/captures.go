package chief

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

// CapturesService reads the caller's own captures. Its routes are authenticated
// but not project-scoped, so no project ID is needed on the Client.
type CapturesService struct {
	client *Client
}

// WithProjectFilter lists only the captures in the given project. One the API
// key doesn't reach lists nothing.
func WithProjectFilter(projectID string) ListOption {
	return func(q url.Values) { q.Set("project_id", projectID) }
}

// WithCaptureFilter lists only the changes of the given capture. One the caller
// can't read has none.
func WithCaptureFilter(captureID string) ListOption {
	return func(q url.Values) { q.Set("capture_id", captureID) }
}

// List returns a cursor page of the caller's captures in every project the API
// key reaches, the last modified first.
func (s *CapturesService) List(ctx context.Context, opts ...ListOption) (*CapturePage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/captures"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page CapturePage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Get returns a capture's metadata. Read what it holds with ListEntries.
func (s *CapturesService) Get(ctx context.Context, captureID string) (*CaptureResponse, error) {
	var capture CaptureResponse
	path := "/v1/captures/" + url.PathEscape(captureID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &capture); err != nil {
		return nil, err
	}
	return &capture, nil
}

// ListEntries returns one page of a capture's entries, oldest first. With no
// cursor it starts at the beginning of the capture. Cursors are entry ids; one
// the capture doesn't hold fails with a 400.
func (s *CapturesService) ListEntries(ctx context.Context, captureID string, opts ...ListOption) (*CaptureEntryPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/captures/" + url.PathEscape(captureID) + "/entries"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page CaptureEntryPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// Changes returns one page of what changed in the caller's captures after
// cursor, which is a CaptureResponse's ChangesCursor or the Cursor of an earlier
// page. Without one the page holds no changes, and its Cursor is now. It takes
// WithLimit, WithProjectFilter and WithCaptureFilter; the limit counts changes
// before the filters leave any out, so a page can be empty with more to come.
// A cursor too old fails with a 410 (see IsGone). Wait is this, polled.
func (s *CapturesService) Changes(ctx context.Context, cursor string, opts ...ListOption) (*CaptureChangesResponse, error) {
	q := url.Values{}
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/captures/changes"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page CaptureChangesResponse
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// GetScreenshot returns a screenshot entry's picture and its content type: a
// JPEG of at most 1280 pixels a side and 1 MiB.
func (s *CapturesService) GetScreenshot(ctx context.Context, captureID, entryID string) ([]byte, string, error) {
	path := "/v1/captures/" + url.PathEscape(captureID) + "/screenshots/" + url.PathEscape(entryID)
	req, err := s.client.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*")

	resp, body, err := s.client.send(req, false)
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// GetShared returns the Markdown a capture's share link serves. The token is
// the whole authorization, so the API key and project are not sent, and neither
// an error nor the debug log holds it. The Markdown does, in the address of
// each screenshot.
func (s *CapturesService) GetShared(ctx context.Context, token string) (string, error) {
	path := "/public-captures/" + url.PathEscape(token) + "/capture.md"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.client.baseURL+path, nil)
	if err != nil {
		// The parse error quotes the URL.
		return "", errors.New("build request: invalid base URL")
	}
	req.Header.Set("Accept", "text/markdown")
	req.Header.Set("User-Agent", defaultUserAgent)

	_, body, err := s.client.send(req, false)
	if urlErr, ok := errors.AsType[*url.Error](err); ok {
		// Its text quotes the URL, so only its cause is kept.
		return "", fmt.Errorf("request failed: %w", urlErr.Err)
	}
	if err != nil {
		return "", err
	}
	return string(body), nil
}
