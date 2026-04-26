import { cn } from "@/lib/utils";
import type { AgentMessage } from "@/lib/api";

interface MessageBubbleProps {
  message: AgentMessage;
  topic?: string | null;
}

function fmtTime(ts: number): string {
  const d = new Date(ts * 1000);
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  const ss = String(d.getSeconds()).padStart(2, "0");
  return `${hh}:${mm}:${ss}`;
}

export function MessageBubble({ message, topic }: MessageBubbleProps) {
  const isUser = message.direction === "in";
  const accent = isUser ? "var(--color-lime)" : "var(--color-magenta)";
  const role = isUser ? "USR" : "MAIN";
  const arrow = isUser ? "▸" : "◂";

  return (
    <div className="flex gap-3 items-start py-2 px-1">
      {/* avatar — square 24px */}
      <div
        className="w-6 h-6 shrink-0 flex items-center justify-center font-display font-bold text-[10px]"
        style={{
          background: isUser
            ? "linear-gradient(135deg, var(--color-lime), var(--color-cyan))"
            : "linear-gradient(135deg, var(--color-magenta), var(--color-cyan))",
          color: "var(--color-bg)",
          clipPath:
            "polygon(20% 0, 100% 0, 100% 80%, 80% 100%, 0 100%, 0 20%)",
        }}
      >
        {isUser ? "Y" : "◆"}
      </div>

      <div className="flex-1 min-w-0">
        {/* header: role · time */}
        <div className="flex items-center gap-2 mb-1">
          <span
            className="font-display font-semibold text-[10px] tracking-hud"
            style={{ color: accent }}
          >
            {arrow} {role}
          </span>
          <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight">
            {fmtTime(message.ts)}
          </span>
          <span className="font-mono text-[9px] text-[var(--color-dim)] uppercase">
            {message.channel}
          </span>
        </div>

        {/* body */}
        <div
          className={cn(
            "font-mono text-[12.5px] leading-[1.55] text-[var(--color-fg)] whitespace-pre-wrap break-words",
            "px-3 py-2 clip-hud-sm border",
            isUser
              ? "bg-[rgba(163,255,78,0.04)] border-[rgba(163,255,78,0.20)]"
              : "bg-[rgba(255,78,214,0.04)] border-[rgba(255,78,214,0.20)]"
          )}
        >
          {message.body || <span className="text-[var(--color-dim)] italic">[vacío]</span>}
        </div>

        {/* topic-pill (placeholder, only on agent replies) */}
        {!isUser && topic && (
          <div className="mt-1.5">
            <span className="topic-pill">▸ {topic}</span>
          </div>
        )}
      </div>
    </div>
  );
}
