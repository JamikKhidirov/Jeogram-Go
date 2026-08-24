-- 000002_message_edit_version.sql
-- Добавляем версионность редактирования и удаление "для всех"
-- (на случай обновления уже существующей базы). Idempotent.
ALTER TABLE messages ADD COLUMN IF NOT EXISTS edit_version integer NOT NULL DEFAULT 0;
ALTER TABLE messages ADD COLUMN IF NOT EXISTS deleted_for_all_at timestamptz;
CREATE INDEX IF NOT EXISTS idx_messages_deleted_for_all ON messages (deleted_for_all_at);
