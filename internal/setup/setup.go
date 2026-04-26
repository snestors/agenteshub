// Package setup contains one-shot helpers (setup-user, etc.).
package setup

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/snestors/agenthub/internal/auth"
	"github.com/snestors/agenthub/internal/config"
	"github.com/snestors/agenthub/internal/domain/models"
	"github.com/snestors/agenthub/internal/store"
)

// User creates or replaces the single AgentHub user, prints the TOTP otpauth URL.
func User(ctx context.Context, cfg *config.Config, repos *store.Repos, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		return fmt.Errorf("bcrypt: %w", err)
	}
	secret, otpURL, err := auth.GenerateTOTPSecret("AgentHub", username)
	if err != nil {
		return fmt.Errorf("totp: %w", err)
	}
	encrypted, err := auth.EncryptAESGCM(cfg.SecretKey, []byte(secret))
	if err != nil {
		return fmt.Errorf("encrypt totp: %w", err)
	}
	user := models.User{
		ID:                  1,
		Username:            username,
		PasswordHash:        string(hash),
		TOTPSecretEncrypted: encrypted,
		CreatedAt:           time.Now().Unix(),
	}
	if err := repos.Auth.CreateUser(ctx, user); err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Println("✓ User created/updated:", username)
	fmt.Println()
	fmt.Println("TOTP otpauth URL (escanealo en Google Authenticator):")
	fmt.Println(otpURL)
	fmt.Println()
	fmt.Println("TOTP secret base32 (manual):")
	fmt.Println(secret)
	if cfg.DevBypassTOTP {
		fmt.Println()
		fmt.Println("⚠ AGENTHUB_DEV_BYPASS_TOTP=true — el TOTP se omite en login (solo dev)")
	}
	return nil
}
