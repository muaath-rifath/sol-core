package user

import (
	"context"
	"time"
)

type contextKey string

const UserKey contextKey = "user"

type User struct {
	ID         string    `json:"id"`
	KeycloakID string    `json:"keycloak_id"`
	Email      string    `json:"email"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(UserKey).(*User)
	return u
}

func WithContext(ctx context.Context, u *User) context.Context {
	return context.WithValue(ctx, UserKey, u)
}
