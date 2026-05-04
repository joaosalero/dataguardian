package httpapi

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const tenantIDKey contextKey = "tenant_id"
const userIDKey contextKey = "user_id"

func withTenantContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := strings.TrimSpace(r.Header.Get("X-Tenant-ID"))
		if tenantID == "" {
			next.ServeHTTP(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), tenantIDKey, tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func tenantIDFromContext(ctx context.Context) *string {
	value, ok := ctx.Value(tenantIDKey).(string)
	if !ok || value == "" {
		return nil
	}
	return &value
}

func withUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func userIDFromContext(ctx context.Context) (int64, bool) {
	value, ok := ctx.Value(userIDKey).(int64)
	return value, ok
}
