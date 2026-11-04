package auth

import (
	"net/http"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

// RequireAdmin rejects users whose role is not admin with 403. Use after Middleware.
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := CurrentUser(r)
		if !ok {
			unauthorized(w, "not authenticated")
			return
		}
		if u.Role != RoleAdmin {
			httpx.Error(w, http.StatusForbidden, httpx.CodeForbidden, "admins only")
			return
		}
		next.ServeHTTP(w, r)
	})
}
