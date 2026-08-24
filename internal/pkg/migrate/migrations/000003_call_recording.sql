-- 000003_call_recording.sql
-- Поля записи звонков (на случай обновления существующей базы). Idempotent.
ALTER TABLE calls ADD COLUMN IF NOT EXISTS recording_url text NOT NULL DEFAULT '';
ALTER TABLE calls ADD COLUMN IF NOT EXISTS recorded_at timestamptz;
