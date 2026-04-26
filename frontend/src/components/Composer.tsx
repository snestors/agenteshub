import * as React from "react";
import { Send, X, Paperclip } from "lucide-react";
import { api, type UploadAttachment, type MessageAttachmentRef } from "@/lib/api";

interface ComposerProps {
  onSend: (
    body: string,
    attachments: MessageAttachmentRef[]
  ) => Promise<void> | void;
  disabled?: boolean;
}

interface Pasted {
  id: number;
  lines: number;
  chars: number;
  text: string;
}

const PASTE_CHAR_THRESHOLD = 800;
const PASTE_LINE_THRESHOLD = 12;
const MAX_VISIBLE_ROWS = 5;

function fmtSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}

export function Composer({ onSend, disabled }: ComposerProps) {
  const [value, setValue] = React.useState("");
  const [pastes, setPastes] = React.useState<Pasted[]>([]);
  const [attachments, setAttachments] = React.useState<UploadAttachment[]>([]);
  const [pending, setPending] = React.useState(false);
  const [uploading, setUploading] = React.useState(0);
  const [dragOver, setDragOver] = React.useState(false);
  const inputRef = React.useRef<HTMLTextAreaElement>(null);
  const fileInputRef = React.useRef<HTMLInputElement>(null);
  const pasteSeqRef = React.useRef(0);
  const localIdRef = React.useRef(0);

  // ── auto-resize: grow up to MAX_VISIBLE_ROWS, then internal scroll ──
  const autoResize = React.useCallback(() => {
    const ta = inputRef.current;
    if (!ta) return;
    // reset before measuring so shrink works too
    ta.style.height = "auto";
    const cs = window.getComputedStyle(ta);
    const lineHeight = parseFloat(cs.lineHeight) || 18;
    const paddingY =
      (parseFloat(cs.paddingTop) || 0) + (parseFloat(cs.paddingBottom) || 0);
    const maxH = lineHeight * MAX_VISIBLE_ROWS + paddingY;
    const next = Math.min(ta.scrollHeight, maxH);
    ta.style.height = `${next}px`;
    ta.style.overflowY = ta.scrollHeight > maxH ? "auto" : "hidden";
  }, []);

  React.useLayoutEffect(() => {
    autoResize();
  }, [value, autoResize]);

  function buildBody(): string {
    const parts: string[] = [];
    if (value.trim().length > 0) parts.push(value.trim());
    if (pastes.length > 0) parts.push(pastes.map((p) => p.text).join("\n\n"));

    if (attachments.length > 0) {
      // If backend supports `attachments` in /api/messages, this inline block
      // is redundant (but harmless). If backend only sees `body`, the agent
      // can still find files via this listing (the daemon adds Read tool).
      const lines = attachments.map(
        (a) =>
          `- ${a.name} (${fmtSize(a.size)}${a.type ? `, ${a.type}` : ""})${
            a.path ? ` → ${a.path}` : ""
          }${a.pending ? " [pending — upload endpoint unavailable]" : ""}`
      );
      parts.push(`--- attachments ---\n${lines.join("\n")}`);
    }
    return parts.join("\n\n");
  }

  function attachmentRefs(): MessageAttachmentRef[] {
    return attachments
      .filter((a) => !a.pending)
      .map((a) => ({ id: a.id, name: a.name, type: a.type, path: a.path }));
  }

  async function submit() {
    const body = buildBody();
    if (!body || pending || disabled) return;
    const refs = attachmentRefs();
    setValue("");
    setPastes([]);
    setAttachments([]);
    pasteSeqRef.current = 0;
    inputRef.current?.focus();
    // ensure height collapses back to single row
    requestAnimationFrame(autoResize);
    setPending(true);
    try {
      await onSend(body, refs);
    } finally {
      setPending(false);
    }
  }

  function handleKey(e: React.KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      void submit();
    }
  }

  function handlePaste(e: React.ClipboardEvent<HTMLTextAreaElement>) {
    const text = e.clipboardData?.getData("text") ?? "";
    if (!text) return;
    const lines = text.split("\n").length;
    if (text.length > PASTE_CHAR_THRESHOLD || lines > PASTE_LINE_THRESHOLD) {
      e.preventDefault();
      pasteSeqRef.current += 1;
      const id = pasteSeqRef.current;
      setPastes((prev) => [...prev, { id, lines, chars: text.length, text }]);
    }
    // else: let the native paste happen
  }

  function removePaste(id: number) {
    setPastes((prev) => prev.filter((p) => p.id !== id));
  }

  async function uploadFile(file: File) {
    setUploading((c) => c + 1);
    try {
      const res = await api.upload(file);
      setAttachments((prev) => [...prev, res]);
    } catch {
      // Backend endpoint missing or failed — degrade gracefully.
      // Keep the file as a "pending" client-side chip so the user can still
      // mention it. The agent will get a textual disclaimer in the body.
      localIdRef.current += 1;
      const fakeId = `local-${Date.now()}-${localIdRef.current}`;
      setAttachments((prev) => [
        ...prev,
        {
          id: fakeId,
          name: file.name,
          size: file.size,
          type: file.type || "application/octet-stream",
          path: "",
          pending: true,
        },
      ]);
    } finally {
      setUploading((c) => Math.max(0, c - 1));
    }
  }

  async function handleFiles(files: FileList | File[] | null) {
    if (!files) return;
    const arr = Array.from(files);
    if (arr.length === 0) return;
    await Promise.all(arr.map((f) => uploadFile(f)));
  }

  function removeAttachment(att: UploadAttachment) {
    setAttachments((prev) => prev.filter((a) => a.id !== att.id));
    if (!att.pending) {
      void api.deleteUpload(att.id).catch(() => {
        /* swallow — UI already removed it */
      });
    }
  }

  function openFilePicker() {
    fileInputRef.current?.click();
  }

  function handleFileInputChange(e: React.ChangeEvent<HTMLInputElement>) {
    void handleFiles(e.target.files);
    // reset so picking the same file twice still triggers change
    e.target.value = "";
  }

  function handleDragOver(e: React.DragEvent<HTMLDivElement>) {
    if (e.dataTransfer?.types?.includes("Files")) {
      e.preventDefault();
      e.stopPropagation();
      if (!dragOver) setDragOver(true);
    }
  }

  function handleDragLeave(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);
  }

  function handleDrop(e: React.DragEvent<HTMLDivElement>) {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);
    void handleFiles(e.dataTransfer?.files ?? null);
  }

  const canSend =
    !pending && !disabled && uploading === 0 && buildBody().length > 0;

  return (
    <div
      className="flex items-end gap-3 px-4 py-3 border-t border-[var(--color-line)]"
      style={{ background: "rgba(0,0,0,0.35)" }}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={handleFileInputChange}
      />

      <div
        className="flex-1 flex flex-col px-3 py-2 clip-hud-sm border transition-colors"
        style={{
          background: dragOver ? "rgba(163, 255, 78, 0.08)" : "rgba(0,0,0,0.45)",
          borderColor: dragOver
            ? "rgba(163, 255, 78, 0.75)"
            : "rgba(255, 78, 214, 0.45)",
        }}
      >
        {(pastes.length > 0 || attachments.length > 0 || uploading > 0) && (
          <div className="flex flex-wrap gap-1 mb-1.5">
            {pastes.map((p) => (
              <span
                key={`paste-${p.id}`}
                className="inline-flex items-center gap-1 px-2 py-0.5 clip-tag font-mono text-[10px] tracking-hud-tight"
                style={{
                  background: "rgba(94, 240, 255, 0.10)",
                  border: "1px solid rgba(94, 240, 255, 0.45)",
                  color: "var(--color-cyan)",
                }}
                title={`${p.chars} chars · ${p.lines} lines`}
              >
                <span>[Pasted #{p.id} +{p.lines} lines]</span>
                <button
                  type="button"
                  onClick={() => removePaste(p.id)}
                  className="opacity-70 hover:opacity-100 transition-opacity cursor-pointer"
                  aria-label={`remove pasted #${p.id}`}
                >
                  <X size={10} strokeWidth={2.2} />
                </button>
              </span>
            ))}

            {attachments.map((a) => (
              <span
                key={`att-${a.id}`}
                className="inline-flex items-center gap-1 px-2 py-0.5 clip-tag font-mono text-[10px] tracking-hud-tight"
                style={{
                  background: a.pending
                    ? "rgba(255, 159, 67, 0.10)"
                    : "rgba(163, 255, 78, 0.10)",
                  border: a.pending
                    ? "1px solid rgba(255, 159, 67, 0.55)"
                    : "1px solid rgba(163, 255, 78, 0.55)",
                  color: a.pending
                    ? "var(--color-orange)"
                    : "var(--color-lime)",
                }}
                title={
                  a.pending
                    ? `${a.name} · ${fmtSize(a.size)} · backend upload no disponible — se enviará nombre`
                    : `${a.name} · ${fmtSize(a.size)}${a.type ? ` · ${a.type}` : ""}`
                }
              >
                <Paperclip size={10} strokeWidth={2.2} />
                <span>
                  {a.name} · {fmtSize(a.size)}
                  {a.pending ? " · pending" : ""}
                </span>
                <button
                  type="button"
                  onClick={() => removeAttachment(a)}
                  className="opacity-70 hover:opacity-100 transition-opacity cursor-pointer"
                  aria-label={`remove attachment ${a.name}`}
                >
                  <X size={10} strokeWidth={2.2} />
                </button>
              </span>
            ))}

            {uploading > 0 && (
              <span
                className="inline-flex items-center gap-1 px-2 py-0.5 clip-tag font-mono text-[10px] tracking-hud-tight animate-pulse"
                style={{
                  background: "rgba(94, 240, 255, 0.10)",
                  border: "1px solid rgba(94, 240, 255, 0.45)",
                  color: "var(--color-cyan)",
                }}
              >
                subiendo {uploading} archivo{uploading > 1 ? "s" : ""}…
              </span>
            )}
          </div>
        )}

        <div className="flex items-start gap-2">
          <button
            type="button"
            onClick={openFilePicker}
            disabled={disabled}
            className="shrink-0 mt-0.5 p-1 clip-tag cursor-pointer transition-opacity hover:opacity-100 opacity-70 disabled:opacity-30 disabled:cursor-not-allowed"
            style={{
              border: "1px solid rgba(163, 255, 78, 0.45)",
              color: "var(--color-lime)",
              background: "rgba(163, 255, 78, 0.06)",
            }}
            aria-label="adjuntar archivo"
            title="adjuntar archivo (o arrastrá uno acá)"
          >
            <Paperclip size={12} strokeWidth={2} />
          </button>
          <span
            className="font-display font-bold text-[14px] pt-1"
            style={{ color: "var(--color-magenta)" }}
          >
            ▸
          </span>
          <textarea
            ref={inputRef}
            value={value}
            disabled={disabled}
            onChange={(e) => setValue(e.target.value)}
            onKeyDown={handleKey}
            onPaste={handlePaste}
            rows={1}
            placeholder={
              pending
                ? "agente pensando… podés escribir el siguiente"
                : dragOver
                ? "soltá los archivos para adjuntar"
                : "habla con el agente…"
            }
            className="flex-1 bg-transparent border-none outline-none resize-none font-mono text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-dim)] py-1"
            style={{ minHeight: 22 }}
          />
          <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight pt-1.5 shrink-0">
            ⏎ ENVIAR
          </span>
        </div>
      </div>

      <button
        onClick={() => void submit()}
        disabled={!canSend}
        className="h-10 px-4 clip-tag font-mono text-[11px] uppercase tracking-hud font-semibold flex items-center gap-2 transition-all disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
        style={{
          background: "rgba(255, 78, 214, 0.12)",
          border: "1px solid var(--color-magenta)",
          color: "var(--color-magenta)",
          boxShadow: pending ? "none" : "0 0 12px rgba(255, 78, 214, 0.35)",
        }}
      >
        <Send size={13} strokeWidth={1.8} />
        {pending ? "..." : "Enviar"}
      </button>
    </div>
  );
}
