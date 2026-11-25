package chief

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const shareToken = "4b1f0000000000000000000000000000000000000000000000000000000000aa"

func newTestClient(t *testing.T, handler http.HandlerFunc, opts ...Option) *Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := New(append(opts, WithAPIKey("pat_key"), WithProjectID("project_a"), WithBaseURL(server.URL))...)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	return c
}

func TestGetScreenshotReturnsTheRawBody(t *testing.T) {
	image := []byte{0xff, 0xd8, 0x00, '{', 0xff, 0xd9}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.EscapedPath() != "/v1/captures/cap%2F1/screenshots/entry1" || r.Header.Get("X-API-Key") != "pat_key" {
			t.Errorf("got %s with key %q", r.URL.EscapedPath(), r.Header.Get("X-API-Key"))
		}
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(image)
	})

	data, contentType, err := c.Captures.GetScreenshot(t.Context(), "cap/1", "entry1")
	if err != nil {
		t.Fatalf("get screenshot: %v", err)
	}
	if !bytes.Equal(data, image) || contentType != "image/jpeg" {
		t.Errorf("got %q as %q, want the image as image/jpeg", data, contentType)
	}
}

func TestGetScreenshotDecodesAnAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = io.WriteString(w, `{"code":"shot.unreadable","humane":"Unreadable.","statusCode":422}`)
	})

	_, _, err := c.Captures.GetScreenshot(t.Context(), "cap", "entry")
	apiErr, ok := IsAPIError(err)
	if !ok || apiErr.Code != "shot.unreadable" || apiErr.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("err = %v, want the decoded envelope", err)
	}
}

func TestGetSharedSendsNoCredential(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/public-captures/"+shareToken+"/capture.md" {
			t.Errorf("path = %q", r.URL.Path)
		}
		for _, name := range []string{"X-API-Key", "X-Project-Id", "Authorization"} {
			if got := r.Header.Get(name); got != "" {
				t.Errorf("%s = %q, want it left out", name, got)
			}
		}
		_, _ = io.WriteString(w, "# Shared\n")
	})

	markdown, err := c.Captures.GetShared(t.Context(), shareToken)
	if err != nil || markdown != "# Shared\n" {
		t.Fatalf("got %q, %v", markdown, err)
	}
}

func TestGetSharedErrorLeavesOutTheToken(t *testing.T) {
	c := newTestClient(t, func(http.ResponseWriter, *http.Request) { panic(http.ErrAbortHandler) })

	_, err := c.Captures.GetShared(t.Context(), shareToken)
	if err == nil || strings.Contains(err.Error(), shareToken) {
		t.Fatalf("err = %v, want one without the token", err)
	}
}

func TestDebugLeavesOutTokensAndImages(t *testing.T) {
	var logged bytes.Buffer
	defer log.SetOutput(log.Writer())
	log.SetOutput(&logged)

	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/public-captures/"):
			_, _ = io.WriteString(w, "![Screen](https://api.example/public-captures/"+shareToken+"/files/a.png)")
		case r.URL.Path == "/v1/links/resolve":
			_, _ = io.WriteString(w, `{"type":"shared_capture","title":"Ideas"}`)
		case strings.Contains(r.URL.Path, "/screenshots/"):
			_, _ = io.WriteString(w, "image-bytes")
		default:
			_, _ = io.WriteString(w, `{"capture_id":"cap","title":"dumped"}`)
		}
	}, WithDebug(true))

	if _, err := c.Captures.GetShared(t.Context(), shareToken); err != nil {
		t.Errorf("get shared: %v", err)
	}
	link, err := c.Links.Resolve(t.Context(), "https://chief.bot/shared/capture/"+shareToken)
	if err != nil || link.Type != LinkTypeSharedCapture {
		t.Errorf("resolve = %+v, %v", link, err)
	}
	if _, _, err := c.Captures.GetScreenshot(t.Context(), "cap", "entry"); err != nil {
		t.Errorf("get screenshot: %v", err)
	}
	if _, err := c.Captures.Get(t.Context(), "cap"); err != nil {
		t.Errorf("get: %v", err)
	}

	if strings.Contains(logged.String(), shareToken) || strings.Contains(logged.String(), "image-bytes") {
		t.Errorf("debug log holds a token or an image:\n%s", logged.String())
	}
	if !strings.Contains(logged.String(), "dumped") {
		t.Error("debug log left out an ordinary response")
	}
}
