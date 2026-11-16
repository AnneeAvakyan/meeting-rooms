package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
)

func (h *AuthHandlers) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		parts := strings.Fields(authorization)
		if len(parts) != 2 || parts[0] != "Bearer" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, errors.New("authorization header is required"))
			return
		}

		id, err := h.tokenValidator.ValidateToken(parts[1])
		if err != nil {
			slog.Warn("invalid token", "error", err)
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, errors.New("invalid or expired token"))
			return
		}

		ctx := context.WithValue(r.Context(), userIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
