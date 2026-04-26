import * as React from "react";
import { Send } from "lucide-react";

interface ComposerProps {
  onSend: (body: string) => Promise<void> | void;
  disabled?: boolean;
}

export function Composer({ onSend, disabled }: ComposerProps) {
  const [value, setValue] = React.useState("");
  const [pending, setPending] = React.useState(false);
  const inputRef = React.useRef<HTMLTextAreaElement>(null);

  async function submit() {
    const trimmed = value.trim();
    if (!trimmed || pending || disabled) return;
    setPending(true);
    try {
      await onSend(trimmed);
      setValue("");
      inputRef.current?.focus();
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

  return (
    <div
      className="flex items-end gap-3 px-4 py-3 border-t border-[var(--color-line)]"
      style={{
        background: "rgba(0,0,0,0.35)",
      }}
    >
      <div
        className="flex-1 flex items-start gap-2 px-3 py-2 clip-hud-sm border"
        style={{
          background: "rgba(0,0,0,0.45)",
          borderColor: "rgba(255, 78, 214, 0.45)",
        }}
      >
        <span
          className="font-display font-bold text-[14px] pt-1"
          style={{ color: "var(--color-magenta)" }}
        >
          ▸
        </span>
        <textarea
          ref={inputRef}
          value={value}
          disabled={pending || disabled}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={handleKey}
          rows={1}
          placeholder={pending ? "esperando respuesta…" : "habla con el agente…"}
          className="flex-1 bg-transparent border-none outline-none resize-none font-mono text-[13px] text-[var(--color-fg)] placeholder:text-[var(--color-dim)] py-1 max-h-32"
          style={{ minHeight: 22 }}
        />
        <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight pt-1.5 shrink-0">
          ⏎ ENVIAR
        </span>
      </div>

      <button
        onClick={() => void submit()}
        disabled={pending || disabled || !value.trim()}
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
