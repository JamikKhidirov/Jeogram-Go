-- 000005_features.sql
-- Кэш-дружелюбные поля, отложенные сообщения, E2EE и групповые звонки.
-- Idempotent — безопасно применять на уже мигрированной базе (PostgreSQL).

ALTER TABLE messages ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'sent';
ALTER TABLE messages ADD COLUMN IF NOT EXISTS scheduled_at timestamptz;

ALTER TABLE chats ADD COLUMN IF NOT EXISTS encryption text NOT NULL DEFAULT 'none';

ALTER TABLE calls ADD COLUMN IF NOT EXISTS mode text NOT NULL DEFAULT 'peer';

CREATE TABLE IF NOT EXISTS e2ee_prekeys (
    id uuid PRIMARY KEY,
    user_id uuid NOT NULL,
    key_id text NOT NULL,
    public_key text NOT NULL,
    signature_key text NOT NULL,
    used boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_e2ee_prekeys_user ON e2ee_prekeys (user_id, used);
