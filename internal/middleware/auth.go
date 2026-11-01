package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"app-finance/internal/models"
)

type contextKey string

const userKey contextKey = "user"

type Authenticator interface {
	Authenticate(token string) (models.User, error)
}

func Auth(a Authenticator, errInvalidToken error) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			token, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !found || token == "" {
				http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
				return
			}

			user, err := a.Authenticate(token)
			if errors.Is(err, errInvalidToken) {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if err != nil {
				slog.Error("failed to authenticate request", "error", err)
				http.Error(w, "failed to authenticate request", http.StatusInternalServerError)
				return
			}

			ctx := context.WithValue(r.Context(), userKey, user)
			next(w, r.WithContext(ctx))
		}
	}
}

func User(ctx context.Context) (models.User, bool) {
	user, ok := ctx.Value(userKey).(models.User)
	return user, ok
}
