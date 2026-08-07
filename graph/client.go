// Package graph is a convenience layer over the generated
// libre-graph-api-go client for adapter services: client construction,
// current-user lookup, drive (space) resolution, recursive traversal
// and driveItem access.
//
// Credentials come from an auth.Source, so the same client works for
// proxy-style adapters that forward each caller's credentials
// (auth.FromRequestContext) and for service-account adapters with one
// configured credential (auth.Static).
//
// A few calls are issued by hand instead of through the generated
// client where the generated surface has gaps (see GetDriveItem and
// Children); they still decode into libregraph model types, so callers
// never see hand-rolled shapes.
package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"

	"github.com/dschmidt/libre-graph-adapter-kit/auth"
)

// ErrNoCredentials is returned when the client's auth.Source yields no
// credentials for a call, e.g. because the auth middleware did not run
// for the inbound request.
var ErrNoCredentials = errors.New("graph: no credentials available")

// Client wraps the generated libregraph client together with the base
// URL and HTTP client it uses, so hand-rolled calls for generated-API
// gaps share connection pool and TLS policy with generated ones.
type Client struct {
	api        *libregraph.APIClient
	baseURL    string // ".../graph", no trailing slash
	httpClient *http.Client
	source     auth.Source
}

type options struct {
	transport http.RoundTripper
}

// Option configures New.
type Option func(*options)

// WithTransport sets the RoundTripper for all requests. Pass a shared
// transport to pool connections with other clients (and to allow
// self-signed certificates in dev setups). Defaults to
// http.DefaultTransport.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// New constructs a client for the OpenCloud instance at baseURL (e.g.
// https://cloud.example.com). The /graph suffix is appended internally
// so the libregraph routes (/graph/v1.0/me, ...) resolve. Credentials
// for each call are drawn from source.
func New(baseURL string, source auth.Source, opts ...Option) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("graph: baseURL is required")
	}
	if source == nil {
		return nil, errors.New("graph: auth source is required")
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/graph")
	if err != nil {
		return nil, fmt.Errorf("graph: invalid baseURL: %w", err)
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}
	httpClient := http.DefaultClient
	if o.transport != nil {
		httpClient = &http.Client{Transport: o.transport}
	}

	cfg := libregraph.NewConfiguration()
	cfg.Servers = libregraph.ServerConfigurations{{URL: u.String()}}
	cfg.HTTPClient = httpClient

	return &Client{
		api:        libregraph.NewAPIClient(cfg),
		baseURL:    u.String(),
		httpClient: httpClient,
		source:     source,
	}, nil
}

// API exposes the underlying generated client for calls this package
// has no helper for. Combine with AuthContext to authenticate them.
func (c *Client) API() *libregraph.APIClient { return c.api }

// AuthContext resolves the call's credentials from the client's source
// and returns a context the generated client's Execute methods accept.
func (c *Client) AuthContext(ctx context.Context) (context.Context, error) {
	creds, ok := c.source.Credentials(ctx)
	if !ok {
		return nil, ErrNoCredentials
	}
	return creds.LibregraphContext(ctx), nil
}

// getJSON issues an authenticated GET against path (relative to the
// /graph base URL) and decodes the JSON response into v. Used for the
// few routes the generated client does not cover.
func (c *Client) getJSON(ctx context.Context, path string, v any) error {
	creds, ok := c.source.Credentials(ctx)
	if !ok {
		return ErrNoCredentials
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	creds.Apply(req)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("graph: GET %s: %w", path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("graph: GET %s: %s", path, resp.Status)
	}
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return fmt.Errorf("graph: decode %s: %w", path, err)
	}
	return nil
}
