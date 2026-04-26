-- 0003_message_activity.sql
-- Stores the agent's activity (thinking + tool calls) for an assistant
-- message so the UI can show a collapsible audit panel under each reply
-- without re-parsing the JSONL each time.

ALTER TABLE wa_messages ADD COLUMN activity TEXT;
