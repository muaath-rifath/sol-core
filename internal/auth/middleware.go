package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const ClaimsKey contextKey = "claims"

type Middleware struct {
	verifier *OIDCVerifier
}

func NewMiddleware(verifier *OIDCVerifier) *Middleware {
	return &Middleware{verifier: verifier}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.verifier.Verify(r.Context(), token)
		if err != nil {
			slog.Warn("auth failed", "error", err)
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func ClaimsFromContext(ctx context.Context) *Claims {
	claims, _ := ctx.Value(ClaimsKey).(*Claims)
	return claims
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}
