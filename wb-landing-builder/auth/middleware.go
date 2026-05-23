package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
)

type contextKey string

const UserIDKey contextKey = "user_id"

func AuthMiddleware(
	authService *AuthService,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			parts := strings.Fields(authHeader)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeJSONError(w, http.StatusUnauthorized, "invalid Authorization header")
				return
			}

			token := parts[1]

			userID, err := authService.GetUserFromToken(token)
			if err != nil {
				if errors.Is(err, ErrInvalidToken) || errors.Is(err, ErrInvalidClaims) || errors.Is(err, ErrUnexpectedSigningMethod) {
					writeJSONError(w, http.StatusUnauthorized, err.Error())
					return
				}
				writeJSONError(w, http.StatusInternalServerError, "internal server error")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
