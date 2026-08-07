package auth

import "context"

type credsCtxKey struct{}

// WithCredentials returns a derived context carrying c. Middleware
// calls this for inbound requests; tests and custom extractors can use
// it directly.
func WithCredentials(ctx context.Context, c Credentials) context.Context {
	return context.WithValue(ctx, credsCtxKey{}, c)
}

// FromContext returns the credentials stored on ctx, or (zero, false)
// when none were set or the stored credentials are incomplete.
func FromContext(ctx context.Context) (Credentials, bool) {
	c, ok := ctx.Value(credsCtxKey{}).(Credentials)
	return c, ok && c.Valid()
}

// Source yields the credentials to use for a call. It abstracts over
// the two adapter flavours: per-request forwarding (FromRequestContext)
// and a configured service account (Static). Clients that take a
// Source work unchanged in both deployment models.
type Source interface {
	// Credentials returns the credentials for the call carried by ctx,
	// or ok == false when none are available.
	Credentials(ctx context.Context) (Credentials, bool)
}

// Static returns a Source that always yields c, for services that
// authenticate with one configured service account.
func Static(c Credentials) Source { return staticSource{c} }

type staticSource struct{ c Credentials }

func (s staticSource) Credentials(context.Context) (Credentials, bool) {
	return s.c, s.c.Valid()
}

// FromRequestContext returns a Source that reads the per-request
// credentials Middleware (or WithCredentials) put on the context, for
// proxy-style services that forward each caller's own credentials.
func FromRequestContext() Source { return ctxSource{} }

type ctxSource struct{}

func (ctxSource) Credentials(ctx context.Context) (Credentials, bool) {
	return FromContext(ctx)
}
