// Package chief is a self-contained Go client for the Chief public REST API.
// It depends only on the standard library so it can be vendored into external
// tools without pulling the platform's internal packages. Wire types are
// copied from the public API surface rather than imported.
package chief

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"
	"strings"
	"time"
)

// DefaultBaseURL is the API root used when no override is configured. The Env*
// constants name the environment variables New reads when the matching option
// is unset.
const (
	DefaultBaseURL = "https://api.storytell.ai"
	EnvAPIKey      = "CHIEF_API_KEY"
	EnvProjectID   = "CHIEF_PROJECT_ID"
	EnvBaseURL     = "CHIEF_BASE_URL"
)

const (
	defaultUserAgent = "chief-go"

	apiKeyHeader    = "X-API-Key"
	projectIDHeader = "X-Project-Id"
)

// Client talks to the Chief public API. Asset routes are project-scoped and
// require a project ID; the raw Do path may target routes that aren't.
type Client struct {
	httpClient         *http.Client
	baseURL            string
	apiKey             string
	projectID          string
	debug              bool
	insecureSkipVerify bool
	// now and sleep are the clock of CapturesService.Wait, which tests replace.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error

	Chats    *ChatsService
	Assets   *AssetsService
	Labels   *LabelsService
	Actions  *ActionsService
	Sessions *SessionsService
	Skills   *SkillsService
	Memories *MemoriesService
	Projects *ProjectsService
	Search   *SearchService
	Captures *CapturesService
	Links    *LinksService
}

// Option configures a Client during New.
type Option func(*Client)

// WithAPIKey sets the Personal Access Token sent as the X-API-Key header.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) { c.apiKey = apiKey }
}

// WithProjectID sets the tenant sent as the X-Project-Id header on asset
// routes.
func WithProjectID(projectID string) Option {
	return func(c *Client) { c.projectID = projectID }
}

// WithBaseURL overrides the API root. Trailing slashes are trimmed.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithDebug dumps requests and responses to the standard logger, except those
// of a call that carries a share link or returns an image.
func WithDebug(debug bool) Option {
	return func(c *Client) { c.debug = debug }
}

// WithInsecureSkipTLSVerify disables TLS certificate verification. Intended
// only for local development against the self-signed dev cert.
func WithInsecureSkipTLSVerify(skip bool) Option {
	return func(c *Client) { c.insecureSkipVerify = skip }
}

// New builds a Client. Options take precedence over the CHIEF_API_KEY,
// CHIEF_PROJECT_ID, and CHIEF_BASE_URL environment variables. An API key is
// required; the project ID may be empty here, but asset calls fail without it.
func New(opts ...Option) (*Client, error) {
	c := &Client{now: time.Now, sleep: sleepContext}
	for _, opt := range opts {
		opt(c)
	}

	// baseURL is defaulted last so an env value isn't mistaken for unset.
	if c.apiKey == "" {
		c.apiKey = os.Getenv(EnvAPIKey)
	}
	if c.projectID == "" {
		c.projectID = os.Getenv(EnvProjectID)
	}
	if c.baseURL == "" {
		if env := os.Getenv(EnvBaseURL); env != "" {
			c.baseURL = strings.TrimRight(env, "/")
		} else {
			c.baseURL = DefaultBaseURL
		}
	}

	if c.apiKey == "" {
		return nil, fmt.Errorf("api key is required: use WithAPIKey or set %s", EnvAPIKey)
	}

	if c.httpClient == nil {
		c.httpClient = &http.Client{}
		if c.insecureSkipVerify {
			c.httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
		}
	}

	c.bindServices()
	return c, nil
}

// bindServices points every service at c. A copy of a Client must call it
// again, or its services keep sending through the original.
func (c *Client) bindServices() {
	c.Chats = &ChatsService{client: c}
	c.Assets = &AssetsService{client: c}
	c.Labels = &LabelsService{client: c}
	c.Actions = &ActionsService{client: c}
	c.Sessions = &SessionsService{client: c}
	c.Skills = &SkillsService{client: c}
	c.Memories = &MemoriesService{client: c}
	c.Projects = &ProjectsService{client: c}
	c.Search = &SearchService{client: c}
	c.Captures = &CapturesService{client: c}
	c.Links = &LinksService{client: c}
}

// ProjectID returns the project sent on project-scoped routes, after the
// CHIEF_PROJECT_ID fallback in New. Empty means none is configured.
func (c *Client) ProjectID() string { return c.projectID }

// ForProject returns a copy of c that sends projectID on project-scoped
// routes. c is left unchanged, so a shared Client can hand out per-call
// copies; each copy shares c's HTTP client and API key.
func (c *Client) ForProject(projectID string) *Client {
	scoped := *c
	scoped.projectID = projectID
	scoped.bindServices()
	return &scoped
}

// Token describes the credential the client authenticates with. An invalid,
// expired, or revoked credential fails with a 401 (see IsUnauthorized).
func (c *Client) Token(ctx context.Context) (*TokenInfo, error) {
	var info TokenInfo
	if _, err := c.Do(ctx, http.MethodGet, "/v1/token", nil, &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// newRequest builds an authenticated JSON request. The auth and tenancy
// headers are set here rather than on a shared transport so the signed upload
// PUT can deliberately omit them.
func (c *Client) newRequest(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set(apiKeyHeader, c.apiKey)
	if c.projectID != "" {
		req.Header.Set(projectIDHeader, c.projectID)
	}

	return req, nil
}

// do sends req and decodes the response. A 2xx response with a non-empty body
// unmarshals into out when out is non-nil.
func (c *Client) do(req *http.Request, out any) (*http.Response, error) {
	resp, body, err := c.send(req, c.debug)
	if err != nil {
		return resp, err
	}

	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return resp, fmt.Errorf("decode response: %w", err)
		}
	}

	return resp, nil
}

// errReadBody marks an answer cut off in its body, which unlike one that came
// whole and doesn't decode is worth asking for again.
var errReadBody = errors.New("read response body")

// send sends req and reads the whole response body. Non-2xx responses decode
// into an *APIError. dump logs both sides in full, so a call that carries a
// share link or returns an image passes false even on a debug client.
func (c *Client) send(req *http.Request, dump bool) (*http.Response, []byte, error) {
	if dump {
		if raw, err := httputil.DumpRequestOut(req, true); err == nil {
			log.Printf("chief request:\n%s", raw)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if dump {
		if raw, err := httputil.DumpResponse(resp, true); err == nil {
			log.Printf("chief response:\n%s", raw)
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp, nil, fmt.Errorf("%w: %w", errReadBody, err)
	}

	if resp.StatusCode >= 400 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		if len(body) > 0 {
			_ = json.Unmarshal(body, apiErr)
		}
		// Decode may leave StatusCode unset if the envelope omits it.
		if apiErr.StatusCode == 0 {
			apiErr.StatusCode = resp.StatusCode
		}
		return resp, nil, apiErr
	}

	return resp, body, nil
}

// Do is the low-level escape hatch: it builds and sends a request and decodes
// the response into out. Pass a *json.RawMessage for out to capture the raw
// body unparsed.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) (*http.Response, error) {
	req, err := c.newRequest(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	return c.do(req, out)
}
