-- 0012_diagram_kinds.sql
-- Diagrams can now be either Mermaid (rendered to SVG client-side) or
-- HTML (rendered inside a sandboxed iframe). The 'kind' column drives the
-- UI; html_content carries the body when kind='html'.

ALTER TABLE diagrams ADD COLUMN kind TEXT NOT NULL DEFAULT 'mermaid';
ALTER TABLE diagrams ADD COLUMN html_content TEXT;
