package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/puzakov/gophermart-loyalty-exam/internal/auth"
	"github.com/puzakov/gophermart-loyalty-exam/internal/domain"
)

type ctxKey string

const userIDKey ctxKey = "userID"

func UserIDFromContext(ctx context.Context) (int64, bool) {
	v := ctx.Value(userIDKey)
	id, ok := v.(int64)
	return id, ok
}

func RequireAuth(tokens *auth.TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := r.Header.Get("Authorization")
			if h == "" {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(h, prefix) {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			raw := strings.TrimSpace(strings.TrimPrefix(h, prefix))
			userID, err := tokens.ParseAccessToken(raw)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func MustUserID(r *http.Request) (int64, error) {
	if id, ok := UserIDFromContext(r.Context()); ok {
		return id, nil
	}
	return 0, domain.ErrUnauthorized
}
