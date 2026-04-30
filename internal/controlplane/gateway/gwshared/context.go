package gwshared

import (
	"context"
	"net/http"
)

type ctxKey string

const tenantKey ctxKey = "tenant_id"

// ContextWithTenant stores the tenant ID in context.
func ContextWithTenant(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, tenantKey, id)
}

// TenantFromCtx extracts the tenant ID from context.
func TenantFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey).(string)
	return v
}

// AuthFunc is a middleware that wraps an http.HandlerFunc with authentication.
type AuthFunc func(http.HandlerFunc) http.HandlerFunc

// LimiterMiddleware wraps an http.HandlerFunc with rate limiting.
type LimiterMiddleware func(http.HandlerFunc) http.HandlerFunc
