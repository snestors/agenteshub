-- 0004_secrets.sql
-- Encrypted vault for tokens / API keys / credentials. value_enc is AES-GCM
-- with the daemon's master key (cfg.SecretKey); never stored in plaintext.

CREATE TABLE IF NOT EXISTS secrets (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  key              TEXT UNIQUE NOT NULL,        -- 'BBVA_API_KEY', 'CLOUDFLARE_TOKEN'
  value_enc        BLOB NOT NULL,                -- AES-GCM ciphertext
  description      TEXT,                         -- humano: "BBVA prod, vence 2027"
  scope            TEXT NOT NULL DEFAULT 'global', -- 'global' | 'project:<id>' | 'agent:<name>'
  expires_at       INTEGER,                      -- opcional, warn cuando se acerca
  created_at       INTEGER NOT NULL,
  updated_at       INTEGER NOT NULL,
  last_accessed_at INTEGER
);

CREATE INDEX IF NOT EXISTS idx_secrets_scope ON secrets(scope);
