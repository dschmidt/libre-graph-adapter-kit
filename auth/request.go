package auth

import (
	"encoding/base64"
	"net/http"
	"strings"
)

// ExtractBearer returns the token from an `Authorization: Bearer ...`
// header, or "" when the header is absent or not the Bearer form.
func ExtractBearer(r *http.Request) string {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !found {
		return ""
	}
	return strings.TrimSpace(raw)
}

// ExtractBasic decodes a standard `Authorization: Basic` header. ok is
// false when the header is absent, malformed, or missing either field.
func ExtractBasic(r *http.Request) (user, pass string, ok bool) {
	raw, found := strings.CutPrefix(r.Header.Get("Authorization"), "Basic ")
	if !found {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return "", "", false
	}
	user, pass, ok = strings.Cut(string(decoded), ":")
	return user, pass, ok && user != "" && pass != ""
}

// FromRequest returns the credentials carried by r's Authorization
// header, preferring Bearer over Basic. The zero Credentials value is
// returned when neither form is present; check Valid before use.
//
// Only the standard header forms are handled here. Protocols with
// their own credential transports (query parameters, form bodies, ...)
// extract those themselves and hand the result to WithCredentials.
func FromRequest(r *http.Request) Credentials {
	if tok := ExtractBearer(r); tok != "" {
		return Bearer(tok)
	}
	if u, p, ok := ExtractBasic(r); ok {
		return Basic(u, p)
	}
	return Credentials{}
}

// Middleware extracts Authorization-header credentials from each
// request and attaches them to the request context for FromContext /
// FromRequestContext consumers. Requests without credentials pass
// through untouched: handlers that require them check FromContext and
// answer in their own protocol's error format.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := FromRequest(r); c.Valid() {
			r = r.WithContext(WithCredentials(r.Context(), c))
		}
		next.ServeHTTP(w, r)
	})
}
