-- 0006_wa_outbox_reply.sql
-- Allow outbox items to reply-to / quote a WhatsApp message by its
-- external StanzaID. The participant is the JID of the original sender,
-- needed for groups; in 1-to-1 it's the same as the conversation jid.

ALTER TABLE wa_outbox ADD COLUMN reply_to TEXT;
ALTER TABLE wa_outbox ADD COLUMN reply_participant TEXT;
