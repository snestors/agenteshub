package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"
)

// EncryptAESGCM encrypts plaintext with a 32-byte master key and prepends the nonce.
func EncryptAESGCM(masterKey, plaintext []byte) ([]byte, error) {
	if len(masterKey) != 32 { return nil, fmt.Errorf("master key must be 32 bytes") }
	block, err := aes.NewCipher(masterKey)
	if err != nil { return nil, fmt.Errorf("new cipher: %w", err) }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, fmt.Errorf("new gcm: %w", err) }
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { return nil, fmt.Errorf("read nonce: %w", err) }
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptAESGCM decrypts ciphertext produced by EncryptAESGCM.
func DecryptAESGCM(masterKey, ciphertext []byte) ([]byte, error) {
	if len(masterKey) != 32 { return nil, fmt.Errorf("master key must be 32 bytes") }
	block, err := aes.NewCipher(masterKey)
	if err != nil { return nil, fmt.Errorf("new cipher: %w", err) }
	gcm, err := cipher.NewGCM(block)
	if err != nil { return nil, fmt.Errorf("new gcm: %w", err) }
	if len(ciphertext) < gcm.NonceSize() { return nil, fmt.Errorf("ciphertext too short") }
	nonce, body := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, body, nil)
	if err != nil { return nil, fmt.Errorf("decrypt: %w", err) }
	return plain, nil
}
