package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/flowcbt/backend/internal/httpx"
)

type ctxKey struct{}

// Middleware memvalidasi Bearer JWT dan menyimpan claims ke context.
func Middleware(m *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if !strings.HasPrefix(h, "Bearer ") {
				httpx.ErrorWriter(w, r, httpx.ErrUnauthorized("UNAUTHORIZED", "token akses wajib (Bearer)"))
				return
			}
			claims, err := m.ParseAccess(strings.TrimPrefix(h, "Bearer "))
			if err != nil {
				httpx.ErrorWriter(w, r, httpx.ErrUnauthorized("TOKEN_INVALID", "token akses tidak valid atau kedaluwarsa"))
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, claims)))
		})
	}
}

// FromContext mengambil claims yang dipasang Middleware.
func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(ctxKey{}).(*Claims)
	return c
}

// RequireRole membatasi endpoint untuk peran tertentu (matriks hak akses 2.1).
// super_admin selalu boleh (console platform).
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromContext(r.Context())
			if c == nil {
				httpx.ErrorWriter(w, r, httpx.ErrUnauthorized("UNAUTHORIZED", "belum masuk"))
				return
			}
			// RBAC: izin disimpulkan dari peran; owner mewarisi kemampuan teacher.
			if c.Role == "super_admin" || allowed["teacher"] && c.Role == "owner" || allowed[c.Role] {
				next.ServeHTTP(w, r)
				return
			}
			httpx.ErrorWriter(w, r, httpx.AppErr(403, "FORBIDDEN", "peran Anda tidak diizinkan untuk aksi ini", nil))
		})
	}
}
