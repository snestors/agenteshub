import * as React from "react";
import { Send, X } from "lucide-react";

interface ComposerProps {
  onSend: (body: string) => Promise<void> | void;
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

export function Composer({ onSend, disabled }: ComposerProps) {
  const [value, setValue] = React.useState("");
  const [pastes, setPastes] = React.useState<Pasted[]>([]);
  const [pending, setPending] = React.useState(false);
  const inputRef = React.useRef<HTMLTextAreaElement>(null);
  const pasteSeqRef = React.useRef(0);

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
    if (pastes.length === 0) return value.trim();
    const blocks = pastes.map((p) => p.text).join("\n\n");
    return value.trim().length > 0
      ? `${value.trim()}\n\n${blocks}`
      : blocks;
  }

  async function submit() {
    const body = buildBody();
    if (!body || pending || disabled) return;
    setValue("");
    setPastes([]);
    pasteSeqRef.current = 0;
    inputRef.current?.focus();
    // ensure height collapses back to single row
    requestAnimationFrame(autoResize);
    setPending(true);
    try {
      await onSend(body);
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

  const canSend = !pending && !disabled && buildBody().length > 0;

  return (
    <div
      className="flex items-end gap-3 px-4 py-3 border-t border-[var(--color-line)]"
      style={{ background: "rgba(0,0,0,0.35)" }}
    >
      <div
        className="flex-1 flex flex-col px-3 py-2 clip-hud-sm border"
        style={{
          background: "rgba(0,0,0,0.45)",
          borderColor: "rgba(255, 78, 214, 0.45)",
        }}
      >
        {pastes.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-1.5">
            {pastes.map((p) => (
              <span
                key={p.id}
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
          </div>
        )}

        <div className="flex items-start gap-2">
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
