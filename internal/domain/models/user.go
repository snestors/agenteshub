package models

// User is the single AgentHub account enforced by auth_users.id = 1.
type User struct {
	ID                  int64  `json:"id"`
	Username            string `json:"username"`
	PasswordHash        string `json:"-"`
	TOTPSecretEncrypted []byte `json:"-"`
	CreatedAt           int64  `json:"created_at"`
	LastLogin           *int64 `json:"last_login,omitempty"`
}
