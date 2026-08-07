// Package auth models the two credential shapes OpenCloud accepts from
// adapter services and moves them between the places an adapter needs
// them: inbound HTTP requests, request contexts, and outgoing calls to
// the Graph API or other OpenCloud endpoints.
//
// The two shapes are:
//
//  1. An OIDC access token, sent as `Authorization: Bearer <token>`.
//     This is what browser-based callers (e.g. OpenCloud web
//     extensions) already hold; adapters forward it verbatim.
//  2. An OpenCloud app token paired with a username, sent as HTTP
//     Basic Auth. App tokens are drop-in replacements for the user's
//     password but cannot identify the user on their own, so the
//     username always travels with them.
//
// Adapters come in two flavours and both are covered by the Source
// interface: proxy-style services forward each caller's own
// credentials (extract with Middleware, consume with FromRequestContext),
// while service-account services use one configured credential for
// everything (Static).
package auth

import (
	"context"
	"net/http"

	libregraph "github.com/opencloud-eu/libre-graph-api-go"
)

// Credentials carries exactly one of the two accepted credential
// shapes: a Bearer token, or a (username, app token) pair. Use the
// Bearer and Basic constructors rather than filling the struct by
// hand.
type Credentials struct {
	// BearerToken is an OIDC access token scoped to OpenCloud. When
	// set, the username/password fields stay empty.
	BearerToken string

	// Username and Password carry the app-token shape: Password is
	// the OpenCloud app token, Username the account it belongs to.
	Username string
	Password string
}

// Bearer returns Credentials wrapping an OIDC access token.
func Bearer(token string) Credentials {
	return Credentials{BearerToken: token}
}

// Basic returns Credentials wrapping a (username, app token) pair.
func Basic(username, appToken string) Credentials {
	return Credentials{Username: username, Password: appToken}
}

// IsBearer reports whether the credentials represent a Bearer token
// rather than a (username, app token) pair.
func (c Credentials) IsBearer() bool { return c.BearerToken != "" }

// Valid reports whether the credentials carry either shape completely:
// a Bearer token, or both username and app token.
func (c Credentials) Valid() bool {
	if c.BearerToken != "" {
		return true
	}
	return c.Username != "" && c.Password != ""
}

// Apply sets the Authorization header on an outgoing request:
// `Bearer <token>` for OIDC access tokens, standard Basic Auth for
// (username, app token) pairs. Use this for hand-rolled HTTP calls;
// calls through the generated libregraph client take LibregraphContext
// instead.
func (c Credentials) Apply(req *http.Request) {
	if c.IsBearer() {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
		return
	}
	req.SetBasicAuth(c.Username, c.Password)
}

// LibregraphContext returns a context carrying the credentials in the
// form the generated libre-graph-api-go client reads them from:
// ContextAccessToken for Bearer tokens (emitted as
// `Authorization: Bearer ...`), ContextBasicAuth otherwise.
func (c Credentials) LibregraphContext(ctx context.Context) context.Context {
	if c.IsBearer() {
		return context.WithValue(ctx, libregraph.ContextAccessToken, c.BearerToken)
	}
	return context.WithValue(ctx, libregraph.ContextBasicAuth, libregraph.BasicAuth{
		UserName: c.Username,
		Password: c.Password,
	})
}
