package schedulerapi

import (
	"context"
	"encoding/json"
	"net/http"

	"yunque-agent/internal/httpctx"
)

func tenantFromCtx(ctx context.Context) string { return httpctx.TenantFromCtx(ctx) }

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
