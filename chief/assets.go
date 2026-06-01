package chief

import (
	"context"
	"crypto/md5" // #nosec G501 -- content fingerprint for dedup, matches GCS object md5; not security
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// AssetsService is the project-scoped asset surface. Every method needs a
// project ID configured on the parent Client.
type AssetsService struct {
	client *Client
}

// ListOption refines a List call's query parameters.
type ListOption func(url.Values)

// WithLimit caps the page size. The server clamps to its own bounds.
func WithLimit(limit int) ListOption {
	return func(q url.Values) { q.Set("limit", strconv.Itoa(limit)) }
}

// WithAfterID pages forward from the given cursor. Mutually exclusive with
// WithBeforeID server-side.
func WithAfterID(id string) ListOption {
	return func(q url.Values) { q.Set("after_id", id) }
}

// WithBeforeID pages backward from the given cursor. Mutually exclusive with
// WithAfterID server-side.
func WithBeforeID(id string) ListOption {
	return func(q url.Values) { q.Set("before_id", id) }
}

// Create mints an asset row and a signed upload URL.
func (s *AssetsService) Create(ctx context.Context, req *CreateAssetRequest) (*CreateAssetResponse, error) {
	var resp CreateAssetResponse
	if _, err := s.client.Do(ctx, http.MethodPost, "/v1/assets", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Complete finalizes an uploaded asset and starts ingest.
func (s *AssetsService) Complete(ctx context.Context, assetID string) (*Asset, error) {
	var asset Asset
	path := "/v1/assets/" + url.PathEscape(assetID) + "/complete"
	if _, err := s.client.Do(ctx, http.MethodPost, path, nil, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// Get returns an asset's current ingest state.
func (s *AssetsService) Get(ctx context.Context, assetID string) (*Asset, error) {
	var asset Asset
	path := "/v1/assets/" + url.PathEscape(assetID)
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// AttachLabel attaches a label by name to an asset. A name with no matching
// label in the project is auto-created; re-attaching an already-attached label
// is a no-op.
func (s *AssetsService) AttachLabel(ctx context.Context, assetID, labelName string) (*LabelSummary, error) {
	var label LabelSummary
	path := "/v1/assets/" + url.PathEscape(assetID) + "/labels"
	if _, err := s.client.Do(ctx, http.MethodPost, path, &AttachLabelRequest{LabelName: labelName}, &label); err != nil {
		return nil, err
	}
	return &label, nil
}

// Update patches an asset's mutable metadata. Nil pointer fields on the request
// leave the corresponding field unchanged.
func (s *AssetsService) Update(ctx context.Context, assetID string, req *UpdateAssetRequest) (*Asset, error) {
	var asset Asset
	path := "/v1/assets/" + url.PathEscape(assetID)
	if _, err := s.client.Do(ctx, http.MethodPost, path, req, &asset); err != nil {
		return nil, err
	}
	return &asset, nil
}

// Delete removes an asset.
func (s *AssetsService) Delete(ctx context.Context, assetID string) error {
	path := "/v1/assets/" + url.PathEscape(assetID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// DetachLabel removes a label from an asset. Detaching a label that isn't
// attached is a no-op.
func (s *AssetsService) DetachLabel(ctx context.Context, assetID, labelID string) error {
	path := "/v1/assets/" + url.PathEscape(assetID) + "/labels/" + url.PathEscape(labelID)
	_, err := s.client.Do(ctx, http.MethodDelete, path, nil, nil)
	return err
}

// List returns a page of assets in the configured project.
func (s *AssetsService) List(ctx context.Context, opts ...ListOption) (*AssetPage, error) {
	q := url.Values{}
	for _, opt := range opts {
		opt(q)
	}
	path := "/v1/assets"
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var page AssetPage
	if _, err := s.client.Do(ctx, http.MethodGet, path, nil, &page); err != nil {
		return nil, err
	}
	return &page, nil
}

// UploadFile runs the full three-step asset flow for a local file: create,
// PUT the bytes to the signed URL, then complete. The bool return is true when
// the content was a dedup hit and no bytes were uploaded. Without a dedup hit
// the returned Asset reflects the post-complete state and may still be
// ingesting; call WaitForReady to block until ingest finishes.
func (s *AssetsService) UploadFile(ctx context.Context, path string) (*Asset, bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false, fmt.Errorf("open file: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return nil, false, fmt.Errorf("stat file: %w", err)
	}

	filename := filepath.Base(path)
	mimeType, err := resolveMimeType(path, file)
	if err != nil {
		return nil, false, err
	}

	md5Hex, err := fileMD5(file)
	if err != nil {
		return nil, false, err
	}

	created, err := s.Create(ctx, &CreateAssetRequest{Filename: filename, MimeType: mimeType, MD5: md5Hex})
	if err != nil {
		return nil, false, err
	}

	if created.AlreadyExists {
		asset, err := s.Get(ctx, created.AssetID)
		return asset, true, err
	}

	if err := s.upload(ctx, created, file, info.Size()); err != nil {
		return nil, false, err
	}

	asset, err := s.Complete(ctx, created.AssetID)
	if err != nil {
		return nil, false, err
	}

	return asset, false, nil
}

// WaitForReady polls the asset until it reaches a terminal status. Returns an
// error when the asset fails or the timeout elapses.
func (s *AssetsService) WaitForReady(ctx context.Context, assetID string, timeout time.Duration) (*Asset, error) {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		asset, err := s.Get(ctx, assetID)
		if err != nil {
			return nil, err
		}
		switch asset.Status {
		case AssetStatusReady:
			return asset, nil
		case AssetStatusFailed:
			return asset, fmt.Errorf("asset %s ingest failed", assetID)
		}

		if time.Now().After(deadline) {
			return asset, fmt.Errorf("asset %s not ready after %s (status %s)", assetID, timeout, asset.Status)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// upload PUTs the file bytes to the signed URL. The request carries only the
// headers the signature was computed over; adding X-API-Key or X-Project-Id
// here would break the signature and the request would be rejected by GCS.
func (s *AssetsService) upload(ctx context.Context, created *CreateAssetResponse, body *os.File, size int64) error {
	req, err := http.NewRequestWithContext(ctx, created.UploadMethod, created.UploadURL, body)
	if err != nil {
		return fmt.Errorf("build upload request: %w", err)
	}
	req.ContentLength = size
	for k, v := range created.UploadHeaders {
		req.Header.Set(k, v)
	}

	resp, err := s.client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("upload bytes: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upload rejected with status %d (signed url expires at %s)",
			resp.StatusCode, created.ExpiresAt.Format(time.RFC3339))
	}
	return nil
}

// fileMD5 hashes the file and rewinds to 0 — the seek side-effect is intentional.
func fileMD5(file *os.File) (string, error) {
	h := md5.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func resolveMimeType(path string, file *os.File) (string, error) {
	if ext := filepath.Ext(path); ext != "" {
		if t := mime.TypeByExtension(ext); t != "" {
			return t, nil
		}
	}

	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return "application/octet-stream", nil
	}
	// Sniffing consumed bytes from the file; rewind so the upload reads from
	// the start.
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind file: %w", err)
	}
	return http.DetectContentType(buf[:n]), nil
}
