package auth

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/muaathrifath/sol-core/internal/user"
)

type contextKey string

const ClaimsKey contextKey = "claims"

// UserUpserter is implemented by user.Repository
type UserUpserter interface {
	Upsert(ctx context.Context, keycloakID, email, name string) (*user.User, bool, error)
}

type Middleware struct {
	verifier  *OIDCVerifier
	users     UserUpserter
	onNewUser func(ctx context.Context, u *user.User)
}

// NewMiddleware creates the auth middleware.
// onNewUser is called (synchronously, within the request context) the first time a user logs in.
// Pass nil if no action is needed on first login.
func NewMiddleware(verifier *OIDCVerifier, users UserUpserter, onNewUser func(context.Context, *user.User)) *Middleware {
	return &Middleware{verifier: verifier, users: users, onNewUser: onNewUser}
}

func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		// Allow internal service token (e.g. from the builder worker)
		internalToken := os.Getenv("INTERNAL_SERVICE_TOKEN")
		if internalToken != "" && token == internalToken {
			u, _, err := m.users.Upsert(r.Context(), "system-builder", "builder@sol.internal", "System Builder")
			if err != nil {
				slog.Error("system user upsert failed", "error", err)
				http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
				return
			}

			claims := &Claims{
				Subject: "system-builder",
				Email:   "builder@sol.internal",
				Name:    "System Builder",
			}
			ctx := context.WithValue(r.Context(), ClaimsKey, claims)
			ctx = user.WithContext(ctx, u)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		claims, err := m.verifier.Verify(r.Context(), token)
		if err != nil {
			slog.Warn("auth failed", "error", err)
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// Enrich missing/garbage profile data from the userinfo endpoint.
		// Zitadel access tokens often omit email/name claims; without this,
		// users are persisted with empty strings or the literal "undefined undefined".
		if claims.Email == "" || claims.Name == "" || claims.Name == "undefined undefined" {
			email, name := m.verifier.Userinfo(r.Context(), token)
			if claims.Email == "" && email != "" {
				claims.Email = email
			}
			if (claims.Name == "" || claims.Name == "undefined undefined") && name != "" {
				claims.Name = name
			}
		}

		u, created, err := m.users.Upsert(r.Context(), claims.Subject, claims.Email, claims.Name)
		if err != nil {
			slog.Error("user upsert failed", "error", err)
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}

		if created && m.onNewUser != nil {
			m.onNewUser(r.Context(), u)
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
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			if token := strings.TrimSpace(r.URL.Query().Get("token")); token != "" {
				return token
			}
		}
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}
