-- Add engine + model to wa_messages so the UI can show "claude · sonnet"
-- per assistant turn. NULL on user messages (channel='in').

ALTER TABLE wa_messages ADD COLUMN engine TEXT;
ALTER TABLE wa_messages ADD COLUMN model TEXT;
