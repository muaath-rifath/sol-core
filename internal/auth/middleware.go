package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/muaathrifath/sol-core/internal/user"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// UserUpserter is implemented by user.Repository
type UserUpserter interface {
	Upsert(ctx context.Context, keycloakID, email, name string) (*user.User, error)
}

type Middleware struct {
	verifier *OIDCVerifier
	users    UserUpserter
}

func NewMiddleware(verifier *OIDCVerifier, users UserUpserter) *Middleware {
	return &Middleware{verifier: verifier, users: users}
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

		u, err := m.users.Upsert(r.Context(), claims.Subject, claims.Email, claims.Name)
		if err != nil {
			slog.Error("user upsert failed", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		ctx = user.WithContext(ctx, u)
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
