package requestid

import "context"

type contextKey struct{}

// With stores the request identity in the context.
func With(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// From returns the request identity, or an empty string when absent.
func From(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}
