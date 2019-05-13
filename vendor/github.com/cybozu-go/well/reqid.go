package well

import "context"

// This file provides utilities for request ID.
// Request ID is passed as context values.

type contextKey string

const (
	// RequestIDContextKey is a context key for request ID.
	RequestIDContextKey contextKey = "request_id"
)

func (k contextKey) String() string {
	return "well: context key: " + string(k)
}

// WithRequestID returns a new context with a request ID as a value.
func WithRequestID(ctx context.Context, reqid string) context.Context {
	return context.WithValue(ctx, RequestIDContextKey, reqid)
}

// BackgroundWithID returns a new background context with an existing
// request ID in ctx, if any.
func BackgroundWithID(ctx context.Context) context.Context {
	id := ctx.Value(RequestIDContextKey)
	ctx = context.Background()
	if id == nil {
		return ctx
	}
	return WithRequestID(ctx, id.(string))
}
