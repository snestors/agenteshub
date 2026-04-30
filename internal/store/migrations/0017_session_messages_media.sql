-- 0017: media support for session_messages
--
-- Until now, session_messages only carried `body`. Project/topic/agent chats
-- live there, so any media generated during a turn (videos, screenshots,
-- voice notes, etc.) had no way to render inline — the MCP send_* tools
-- could only target wa_messages (web/wa) and rejected everything else.
--
-- Adding the same trio that wa_messages uses keeps the renderer parity:
-- MessageBubble already understands media_type/media_path/media_caption.

ALTER TABLE session_messages ADD COLUMN media_type    TEXT;
ALTER TABLE session_messages ADD COLUMN media_path    TEXT;
ALTER TABLE session_messages ADD COLUMN media_caption TEXT;
