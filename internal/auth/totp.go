package auth

import (
	"fmt"

	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP key and returns the raw secret and otpauth URL.
func GenerateTOTPSecret(issuer, account string) (secret string, url string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{Issuer: issuer, AccountName: account})
	if err != nil { return "", "", fmt.Errorf("generate totp: %w", err) }
	return key.Secret(), key.URL(), nil
}

// BuildTOTPKey builds an OTP key URL for QR provisioning from an existing secret.
func BuildTOTPKey(issuer, account, secret string) (string, error) {
	key, err := otp.NewKeyFromURL(fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s", issuer, account, secret, issuer))
	if err != nil { return "", fmt.Errorf("build totp key: %w", err) }
	return key.URL(), nil
}

// ValidateTOTP validates a TOTP code against a raw secret.
func ValidateTOTP(code, secret string) bool { return totp.Validate(code, secret) }
