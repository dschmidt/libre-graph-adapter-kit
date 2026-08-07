package proxy

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/dschmidt/libre-graph-adapter-kit/auth"
)

// upstream records the last request it saw and answers with canned
// headers, so tests can assert both directions of the whitelist.
type upstream struct {
	t        *testing.T
	lastReq  *http.Request
	status   int
	headers  http.Header
	body     string
	lastPath string
}

func (u *upstream) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u.lastReq = r.Clone(r.Context())
		u.lastPath = r.URL.RequestURI()
		for k, vs := range u.headers {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(u.status)
		_, _ = io.WriteString(w, u.body)
	}
}

func TestDownloadPassesRangeAndWhitelistsHeaders(t *testing.T) {
	up := &upstream{
		t:      t,
		status: http.StatusPartialContent,
		headers: http.Header{
			"Content-Type":  {"audio/flac"},
			"Content-Range": {"bytes 100-199/1000"},
			"Accept-Ranges": {"bytes"},
			"Etag":          {`"v1"`},
			// Junk that must not reach the client:
			"Dav":           {"1, 2"},
			"Set-Cookie":    {"session=abc"},
			"Ms-Author-Via": {"DAV"},
		},
		body: "0123456789",
	}
	srv := httptest.NewServer(up.handler())
	defer srv.Close()

	r := httptest.NewRequest(http.MethodGet, "/rest/stream?u=alice&p=secret", nil)
	r.Header.Set("Range", "bytes=100-199")
	r.Header.Set("If-Range", `"v1"`)
	r.Header.Set("Cookie", "client=cookie")
	r.Header.Set("Authorization", "Basic Zm9vOmJhcg==")
	w := httptest.NewRecorder()

	Download(w, r, srv.URL+"/signed/path%20x?sig=abc")

	// Upstream request: exact URL, whitelisted headers only.
	if up.lastPath != "/signed/path%20x?sig=abc" {
		t.Fatalf("upstream saw %q", up.lastPath)
	}
	if got := up.lastReq.Header.Get("Range"); got != "bytes=100-199" {
		t.Fatalf("Range not forwarded, got %q", got)
	}
	if got := up.lastReq.Header.Get("If-Range"); got != `"v1"` {
		t.Fatalf("If-Range not forwarded, got %q", got)
	}
	for _, k := range []string{"Cookie", "Authorization"} {
		if v := up.lastReq.Header.Get(k); v != "" {
			t.Fatalf("%s leaked upstream: %q", k, v)
		}
	}

	// Client response: status, body and whitelisted headers only.
	resp := w.Result()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if body, _ := io.ReadAll(resp.Body); string(body) != "0123456789" {
		t.Fatalf("body = %q", body)
	}
	if got := resp.Header.Get("Content-Range"); got != "bytes 100-199/1000" {
		t.Fatalf("Content-Range = %q", got)
	}
	if got := resp.Header.Get("ETag"); got != `"v1"` {
		t.Fatalf("ETag = %q", got)
	}
	for _, k := range []string{"Dav", "Set-Cookie", "Ms-Author-Via"} {
		if v := resp.Header.Get(k); v != "" {
			t.Fatalf("%s leaked to client: %q", k, v)
		}
	}
}

func TestDownloadForcesGETUpstream(t *testing.T) {
	up := &upstream{t: t, status: http.StatusOK, body: "x"}
	srv := httptest.NewServer(up.handler())
	defer srv.Close()

	r := httptest.NewRequest(http.MethodPost, "/rest/stream", nil)
	Download(httptest.NewRecorder(), r, srv.URL+"/f")
	if up.lastReq.Method != http.MethodGet {
		t.Fatalf("upstream method = %s, want GET", up.lastReq.Method)
	}

	r = httptest.NewRequest(http.MethodHead, "/rest/stream", nil)
	Download(httptest.NewRecorder(), r, srv.URL+"/f")
	if up.lastReq.Method != http.MethodHead {
		t.Fatalf("upstream method = %s, want HEAD", up.lastReq.Method)
	}
}

func TestDownloadStatusPassthrough(t *testing.T) {
	for _, status := range []int{
		http.StatusOK,
		http.StatusNotModified,
		http.StatusRequestedRangeNotSatisfiable,
		http.StatusNotFound,
	} {
		up := &upstream{t: t, status: status}
		srv := httptest.NewServer(up.handler())
		w := httptest.NewRecorder()
		Download(w, httptest.NewRequest(http.MethodGet, "/", nil), srv.URL+"/f")
		srv.Close()
		if w.Code != status {
			t.Fatalf("status %d came back as %d", status, w.Code)
		}
	}
}

func TestWithCredentials(t *testing.T) {
	up := &upstream{t: t, status: http.StatusOK}
	srv := httptest.NewServer(up.handler())
	defer srv.Close()

	Download(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil),
		srv.URL+"/f", WithCredentials(auth.Basic("alice", "app-token")))

	user, pass, ok := up.lastReq.BasicAuth()
	if !ok || user != "alice" || pass != "app-token" {
		t.Fatalf("upstream auth = (%q, %q, %v)", user, pass, ok)
	}
}

func TestErrorHandling(t *testing.T) {
	// Invalid target: error handler runs, nothing else written.
	var handled error
	w := httptest.NewRecorder()
	Download(w, httptest.NewRequest(http.MethodGet, "/", nil), "::not-a-url",
		WithErrorHandler(func(w http.ResponseWriter, _ *http.Request, err error) {
			handled = err
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
	if handled == nil || w.Code != http.StatusServiceUnavailable {
		t.Fatalf("custom handler not used: err=%v code=%d", handled, w.Code)
	}

	// Relative URL is also rejected before any upstream contact.
	handled = nil
	Download(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), "/no/host",
		WithErrorHandler(func(http.ResponseWriter, *http.Request, error) {
			handled = io.EOF // marker
		}))
	if handled == nil {
		t.Fatal("relative target not rejected")
	}

	// Unreachable upstream: default handler answers 502.
	srv := httptest.NewServer(http.NotFoundHandler())
	dead, _ := url.Parse(srv.URL)
	srv.Close() // now nothing listens there
	w = httptest.NewRecorder()
	Download(w, httptest.NewRequest(http.MethodGet, "/", nil), dead.String()+"/f")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("default error handler wrote %d, want 502", w.Code)
	}
}
