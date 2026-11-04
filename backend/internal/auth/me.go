package auth

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/AlikoYe/tutorhub/backend/internal/platform/httpx"
)

// SpecialistLookup returns the current user's own specialist profile summary, or nil.
type SpecialistLookup interface {
	MySpecialist(ctx context.Context, userID int64) (any, error)
}

type meResponse struct {
	TelegramID   int64   `json:"telegram_id"`
	Name         string  `json:"name"`
	Username     *string `json:"username"`
	LanguageCode *string `json:"language_code"`
	Role         string  `json:"role"`
	IsAdmin      bool    `json:"is_admin"`
	Specialist   any     `json:"specialist"`
}

// MeHandler serves GET /api/me. specialists may be nil (no specialist module yet).
func MeHandler(specialists SpecialistLookup, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := CurrentUser(r)
		if !ok {
			unauthorized(w, "not authenticated")
			return
		}
		resp := meResponse{
			TelegramID:   user.ID,
			Name:         user.DisplayName(),
			Username:     nullable(user.Username),
			LanguageCode: nullable(user.LanguageCode),
			Role:         user.Role,
			IsAdmin:      user.Role == RoleAdmin,
		}
		if specialists != nil {
			s, err := specialists.MySpecialist(r.Context(), user.ID)
			if err != nil {
				log.ErrorContext(r.Context(), "me: specialist", slog.Any("err", err))
				httpx.Internal(w)
				return
			}
			resp.Specialist = s
		}
		httpx.JSON(w, http.StatusOK, resp)
	}
}
