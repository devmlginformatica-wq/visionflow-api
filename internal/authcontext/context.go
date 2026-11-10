package authcontext

import "context"

type contextKey string

const (
	userIDKey  contextKey = "user_id"
	profileKey contextKey = "profile"
)

func WithIdentity(ctx context.Context, userID int, profile string) context.Context {
	ctx = context.WithValue(ctx, userIDKey, userID)
	return context.WithValue(ctx, profileKey, profile)
}

func UserID(ctx context.Context) int {
	userID, _ := ctx.Value(userIDKey).(int)
	return userID
}

func Profile(ctx context.Context) string {
	profile, _ := ctx.Value(profileKey).(string)
	return profile
}
