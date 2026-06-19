package gwshared

import (
	"context"
	"encoding/json"
	"net/http"

	"yunque-agent/internal/httpctx"
)

// ContextWithTenant stores the tenant ID in context.
func ContextWithTenant(ctx context.Context, id string) context.Context {
	return httpctx.ContextWithTenant(ctx, id)
}

// TenantFromCtx extracts the tenant ID from context.
func TenantFromCtx(ctx context.Context) string {
	return httpctx.TenantFromCtx(ctx)
}

// AuthFunc is a middleware that wraps an http.HandlerFunc with authentication.
type AuthFunc func(http.HandlerFunc) http.HandlerFunc

// LimiterMiddleware wraps an http.HandlerFunc with rate limiting.
type LimiterMiddleware func(http.HandlerFunc) http.HandlerFunc

// WriteJSON writes a JSON response with 200 OK.
func WriteJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// WriteJSONStatus writes a JSON response with the given status code.
func WriteJSONStatus(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
