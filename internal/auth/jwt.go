package auth

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/agenteshub/agenteshub/internal/config"
)

// RevocationStore checks and records JWT revocation state.
type RevocationStore interface {
	IsJTIRevoked(ctx context.Context, jti string) (bool, error)
	RevokeJTI(ctx context.Context, jti string, expiresAt int64) error
}

// Claims are AgentHub JWT claims.
type Claims struct {
	RefreshUntil int64 `json:"refresh_until"`
	jwt.RegisteredClaims
}

// TokenService issues and verifies AgentHub JWTs.
type TokenService struct {
	cfg   *config.Config
	store RevocationStore
}

// NewTokenService creates a JWT service.
func NewTokenService(cfg *config.Config, store RevocationStore) *TokenService { return &TokenService{cfg: cfg, store: store} }

// Issue signs a new HS256 token for userID and jti. If jti is empty, a UUID is generated.
func (s *TokenService) Issue(userID int64, jti string) (string, error) {
	if jti == "" { jti = uuid.NewString() }
	now := time.Now()
	claims := Claims{RefreshUntil: now.Add(s.cfg.JWTRefreshTTL).Unix(), RegisteredClaims: jwt.RegisteredClaims{Subject: strconv.FormatInt(userID, 10), ID: jti, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.cfg.JWTTTL))}}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims).SignedString(s.cfg.JWTSecret)
	if err != nil { return "", fmt.Errorf("sign jwt: %w", err) }
	return token, nil
}

// Verify validates a token signature and returns its claims.
func (s *TokenService) Verify(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 { return nil, fmt.Errorf("unexpected signing method %s", token.Method.Alg()) }
		return s.cfg.JWTSecret, nil
	})
	if err != nil { return nil, fmt.Errorf("parse jwt: %w", err) }
	if !token.Valid { return nil, fmt.Errorf("invalid jwt") }
	return claims, nil
}

// Revoke records the token claims as revoked.
func (s *TokenService) Revoke(ctx context.Context, claims *Claims) error {
	if claims.ExpiresAt == nil { return fmt.Errorf("missing token expiry") }
	return s.store.RevokeJTI(ctx, claims.ID, claims.ExpiresAt.Unix())
}

// UserID parses the subject claim.
func (c *Claims) UserID() (int64, error) {
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil { return 0, fmt.Errorf("parse subject: %w", err) }
	return id, nil
}
