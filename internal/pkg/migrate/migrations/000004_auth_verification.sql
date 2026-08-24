-- 000004_auth_verification.sql
-- Поля верификации/телефона/статуса и таблица кодов подтверждения.
-- Idempotent — безопасно применять на уже мигрированной базе.
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone text NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone_verified boolean NOT NULL DEFAULT false;
ALTER TABLE users ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
ALTER TABLE users ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'user';

CREATE TABLE IF NOT EXISTS verification_codes (
    id uuid PRIMARY KEY,
    user_id uuid,
    target text NOT NULL,
    purpose text NOT NULL,
    code text NOT NULL,
    expires_at timestamptz NOT NULL,
    used boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_verification_target ON verification_codes (target, purpose);
