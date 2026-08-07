// Package proxy streams a driveItem's content from OpenCloud to the
// adapter's client, passing Range and conditional request headers
// through so media players can seek into files and revalidate caches.
//
// It is a thin, whitelist-based configuration of
// net/http/httputil.ReverseProxy: instead of forwarding everything
// except hop-by-hop headers (the ReverseProxy default), both
// directions carry only the headers a download needs. Nothing else
// from the foreign protocol's request (cookies, query credentials,
// auth headers) reaches OpenCloud, and nothing from OpenCloud's
// response (WebDAV, CORS or cookie headers) reaches the client.
//
// Forwarded request headers: Range, If-Range, If-Modified-Since,
// If-None-Match. Forwarded response headers: Content-Type,
// Content-Length, Content-Range, Accept-Ranges, ETag, Last-Modified.
// Status codes pass through untouched, including 206, 304 and 416.
//
// The target is typically a pre-signed download URL (the driveItem's
// `@microsoft.graph.downloadUrl`), which needs no credentials. For
// plain WebDAV URLs, WithCredentials attaches the caller's OpenCloud
// credentials to the outgoing request.
package proxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/dschmidt/libre-graph-adapter-kit/auth"
)

// passRequestHeaders are the inbound headers forwarded upstream: the
// cache/range negotiation set a media client uses to seek and
// revalidate.
var passRequestHeaders = []string{
	"Range", "If-Range", "If-Modified-Since", "If-None-Match",
}

// passResponseHeaders are the upstream headers forwarded back: the set
// a media client needs to play, seek and cache the content.
var passResponseHeaders = []string{
	"Content-Type", "Content-Length", "Content-Range",
	"Accept-Ranges", "ETag", "Last-Modified",
}

type options struct {
	transport    http.RoundTripper
	creds        auth.Credentials
	hasCreds     bool
	errorHandler func(http.ResponseWriter, *http.Request, error)
}

// Option configures a Download call.
type Option func(*options)

// WithTransport sets the RoundTripper for upstream requests. Pass a
// shared transport so connections are pooled across calls (and to
// allow self-signed certificates in dev setups). Defaults to
// http.DefaultTransport.
func WithTransport(rt http.RoundTripper) Option {
	return func(o *options) { o.transport = rt }
}

// WithCredentials attaches OpenCloud credentials to the upstream
// request. Only needed for targets that require authentication (plain
// WebDAV URLs); pre-signed download URLs carry their own authorization.
func WithCredentials(c auth.Credentials) Option {
	return func(o *options) { o.creds, o.hasCreds = c, true }
}

// WithErrorHandler sets the handler invoked when the target URL is
// unusable or the upstream request fails before any response bytes
// were written, so adapters can answer in their own protocol's error
// format. The default writes a plain-text 502. Upstream HTTP error
// responses (4xx/5xx) are not routed here; they pass through to the
// client like any other status.
func WithErrorHandler(fn func(http.ResponseWriter, *http.Request, error)) Option {
	return func(o *options) { o.errorHandler = fn }
}

// Download proxies the content behind target to w, forwarding r's
// range/conditional request headers and the upstream's download
// response headers per the package whitelists. The upstream method is
// GET (HEAD when r is a HEAD request) regardless of r's method, so
// protocols that accept POST requests for downloads can pass r
// through unchanged.
func Download(w http.ResponseWriter, r *http.Request, target string, opts ...Option) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.errorHandler == nil {
		o.errorHandler = func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "upstream download failed", http.StatusBadGateway)
		}
	}

	u, err := url.Parse(target)
	if err != nil || u.Scheme == "" || u.Host == "" {
		o.errorHandler(w, r, fmt.Errorf("proxy: invalid target URL %q: %w", target, err))
		return
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Send the exact target URL; deliberately not SetURL,
			// which would join the inbound request path onto it.
			// Replacing the URL wholesale also drops the inbound
			// query string (which may carry protocol credentials).
			pr.Out.URL = u
			pr.Out.Host = ""

			// Downloads are GETs upstream even when the foreign
			// protocol delivered the request as POST.
			if pr.Out.Method != http.MethodHead {
				pr.Out.Method = http.MethodGet
			}
			pr.Out.Body = nil
			pr.Out.ContentLength = 0

			h := make(http.Header, len(passRequestHeaders)+1)
			for _, k := range passRequestHeaders {
				if v := pr.In.Header.Get(k); v != "" {
					h.Set(k, v)
				}
			}
			pr.Out.Header = h
			if o.hasCreds {
				o.creds.Apply(pr.Out)
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			h := make(http.Header, len(passResponseHeaders))
			for _, k := range passResponseHeaders {
				if v := resp.Header.Get(k); v != "" {
					h.Set(k, v)
				}
			}
			resp.Header = h
			return nil
		},
		Transport:    o.transport,
		ErrorHandler: o.errorHandler,
	}
	rp.ServeHTTP(w, r)
}
