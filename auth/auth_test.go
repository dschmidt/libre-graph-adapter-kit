package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"testing"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

func TestCredentialsValid(t *testing.T) {
	cases := []struct {
		name  string
		creds Credentials
		valid bool
	}{
		{"zero", Credentials{}, false},
		{"bearer", Bearer("tok"), true},
		{"basic", Basic("alice", "app-token"), true},
		{"user only", Credentials{Username: "alice"}, false},
		{"password only", Credentials{Password: "app-token"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.creds.Valid(); got != tc.valid {
				t.Fatalf("Valid() = %v, want %v", got, tc.valid)
			}
		})
	}
}

func TestApply(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	Bearer("tok").Apply(req)
	if got := req.Header.Get("Authorization"); got != "Bearer tok" {
		t.Fatalf("bearer Apply set %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	Basic("alice", "secret").Apply(req)
	u, p, ok := req.BasicAuth()
	if !ok || u != "alice" || p != "secret" {
		t.Fatalf("basic Apply set (%q, %q, %v)", u, p, ok)
	}
}

func TestLibregraphContext(t *testing.T) {
	ctx := Bearer("tok").LibregraphContext(context.Background())
	if got := ctx.Value(libregraph.ContextAccessToken); got != "tok" {
		t.Fatalf("ContextAccessToken = %v", got)
	}

	ctx = Basic("alice", "secret").LibregraphContext(context.Background())
	ba, ok := ctx.Value(libregraph.ContextBasicAuth).(libregraph.BasicAuth)
	if !ok || ba.UserName != "alice" || ba.Password != "secret" {
		t.Fatalf("ContextBasicAuth = %#v (ok=%v)", ba, ok)
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithCredentials(context.Background(), Basic("alice", "secret"))
	c, ok := FromContext(ctx)
	if !ok || c.Username != "alice" || c.Password != "secret" {
		t.Fatalf("FromContext = %#v (ok=%v)", c, ok)
	}

	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("FromContext on empty context reported ok")
	}

	// Incomplete credentials stored on the context must not surface.
	ctx = WithCredentials(context.Background(), Credentials{Username: "alice"})
	if _, ok := FromContext(ctx); ok {
		t.Fatal("FromContext surfaced invalid credentials")
	}
}

func basicHeader(user, pass string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+pass))
}

func TestFromRequest(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   Credentials
	}{
		{"none", "", Credentials{}},
		{"bearer", "Bearer tok", Bearer("tok")},
		{"bearer padded", "Bearer   tok  ", Bearer("tok")},
		{"basic", basicHeader("alice", "secret"), Basic("alice", "secret")},
		{"basic with colon in password", basicHeader("alice", "se:cret"), Basic("alice", "se:cret")},
		{"basic malformed base64", "Basic ???", Credentials{}},
		{"basic missing colon", "Basic " + base64.StdEncoding.EncodeToString([]byte("alice")), Credentials{}},
		{"basic empty user", basicHeader("", "secret"), Credentials{}},
		{"basic empty password", basicHeader("alice", ""), Credentials{}},
		{"other scheme", "Digest abc", Credentials{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			if got := FromRequest(req); got != tc.want {
				t.Fatalf("FromRequest = %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestMiddleware(t *testing.T) {
	var seen Credentials
	var seenOK bool
	h := Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, seenOK = FromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer tok")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !seenOK || !seen.IsBearer() || seen.BearerToken != "tok" {
		t.Fatalf("middleware attached %#v (ok=%v)", seen, seenOK)
	}

	// Without credentials the request passes through with none attached.
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if seenOK {
		t.Fatal("middleware attached credentials to a bare request")
	}
}

func TestSources(t *testing.T) {
	static := Static(Basic("svc", "token"))
	if c, ok := static.Credentials(context.Background()); !ok || c.Username != "svc" {
		t.Fatalf("Static source = %#v (ok=%v)", c, ok)
	}
	if _, ok := Static(Credentials{}).Credentials(context.Background()); ok {
		t.Fatal("Static source with invalid credentials reported ok")
	}

	src := FromRequestContext()
	ctx := WithCredentials(context.Background(), Bearer("tok"))
	if c, ok := src.Credentials(ctx); !ok || c.BearerToken != "tok" {
		t.Fatalf("FromRequestContext source = %#v (ok=%v)", c, ok)
	}
	if _, ok := src.Credentials(context.Background()); ok {
		t.Fatal("FromRequestContext source on empty context reported ok")
	}
}
