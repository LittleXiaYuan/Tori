package httpctx

import "context"

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
