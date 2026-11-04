package auth

import (
	"context"
	"net/http"
)

type ctxKey struct{}

// WithUser returns a copy of ctx carrying the authenticated user.
func WithUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// UserFromContext returns the authenticated user, if any.
func UserFromContext(ctx context.Context) (User, bool) {
	u, ok := ctx.Value(ctxKey{}).(User)
	return u, ok
}

// CurrentUser is shorthand for UserFromContext(r.Context()).
func CurrentUser(r *http.Request) (User, bool) {
	return UserFromContext(r.Context())
}
