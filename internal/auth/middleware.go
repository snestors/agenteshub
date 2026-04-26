package auth

import (
	"context"
	"encoding/json"
	"net/http"
)

type contextKey string

const userIDKey contextKey = "agenthub_user_id"
const claimsKey contextKey = "agenthub_claims"

// RequireJWT validates the agenthub_token cookie, checks revocation, and stores user ID in context.
func (s *TokenService) RequireJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("agenthub_token")
		if err != nil { unauthorized(w); return }
		claims, err := s.Verify(cookie.Value)
		if err != nil { unauthorized(w); return }
		revoked, err := s.store.IsJTIRevoked(r.Context(), claims.ID)
		if err != nil || revoked { unauthorized(w); return }
		uid, err := claims.UserID()
		if err != nil { unauthorized(w); return }
		ctx := context.WithValue(r.Context(), userIDKey, uid)
		ctx = context.WithValue(ctx, claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// UserIDFromContext returns the authenticated user ID.
func UserIDFromContext(ctx context.Context) (int64, bool) {
	v, ok := ctx.Value(userIDKey).(int64)
	return v, ok
}

// ClaimsFromContext returns JWT claims stored by RequireJWT.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	v, ok := ctx.Value(claimsKey).(*Claims)
	return v, ok
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": "unauthorized", "message": "unauthorized"}})
}
