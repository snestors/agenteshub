-- 0007_wa_messages_external_id.sql
-- Persist the WhatsApp StanzaID of each row so the agent can quote it back
-- on a later send_message reply_to. wa_messages.id is our internal
-- AUTOINCREMENT pk; external_id is what WhatsApp uses on the wire.

ALTER TABLE wa_messages ADD COLUMN external_id TEXT;
CREATE INDEX IF NOT EXISTS idx_wa_messages_external ON wa_messages(external_id);
