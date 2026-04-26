import { cn } from "@/lib/utils";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import { Brain, ChevronRight, Loader2, CheckCircle2 } from "lucide-react";

export type ToolStatus = "running" | "ok" | "error";

export interface ToolCall {
  /** stable id within a turn — use seq number from the envelope */
  id: string;
  name: string;
  args?: unknown;
  status: ToolStatus;
  /** result preview (first ~200 chars) */
  resultPreview?: string;
}

export interface GhostBubbleData {
  /** synthetic id — `stream-<session>-<turn>` */
  id: string;
  /** accumulated thinking text (kind=thinking) */
  thinking: string;
  /** accumulated final text (kind=text) — what the model is "saying" */
  text: string;
  /** tool calls in order */
  tools: ToolCall[];
  /** true while the user is waiting for the first chunk (no stream yet) */
  pending?: boolean;
  /** true once 'final' arrived — kept until the persisted message reaches us */
  done?: boolean;
}

// minimal markdown components (reused styling, lighter version)
const mdComponents: Components = {
  p: ({ children }) => (
    <p className="my-1 first:mt-0 last:mb-0 leading-[1.55]">{children}</p>
  ),
  code: ({ className, children, ...rest }) => {
    const inline = !className;
    if (inline) {
      return (
        <code
          className="font-mono text-[11.5px] px-1 py-[1px] rounded"
          style={{
            color: "var(--color-cyan)",
            background: "rgba(94, 240, 255, 0.10)",
          }}
          {...rest}
        >
          {children}
        </code>
      );
    }
    return (
      <code className={cn("font-mono text-[11.5px]", className)} {...rest}>
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <pre
      className="my-2 px-3 py-2 overflow-x-auto clip-hud-sm font-mono text-[11.5px] leading-[1.5]"
      style={{
        background: "rgba(0,0,0,0.55)",
        border: "1px solid var(--color-line)",
        color: "var(--color-fg)",
      }}
    >
      {children}
    </pre>
  ),
};

function formatArgs(args: unknown): string {
  if (args === undefined || args === null) return "";
  if (typeof args === "string") return args;
  try {
    return JSON.stringify(args);
  } catch {
    return String(args);
  }
}

function ToolCard({ call }: { call: ToolCall }) {
  const statusColor =
    call.status === "ok"
      ? "var(--color-lime)"
      : call.status === "error"
      ? "var(--color-danger)"
      : "var(--color-orange)";

  const argsStr = formatArgs(call.args);

  return (
    <div
      className={cn(
        "my-1.5 clip-hud-sm font-mono text-[11px]",
        call.status === "running" && "animate-pulse"
      )}
      style={{
        background: "rgba(255, 159, 67, 0.06)",
        border: `1px solid ${statusColor}`,
        color: "var(--color-fg)",
      }}
    >
      <div
        className="flex items-center gap-2 px-2 py-1 border-b"
        style={{ borderColor: "rgba(255, 159, 67, 0.25)" }}
      >
        <ChevronRight size={12} style={{ color: statusColor }} />
        <span
          className="font-display font-semibold tracking-hud-tight"
          style={{ color: statusColor }}
        >
          {call.name}
        </span>
        <span className="ml-auto flex items-center gap-1 text-[9px] tracking-hud uppercase opacity-80">
          {call.status === "running" && <Loader2 size={10} className="animate-spin" />}
          {call.status === "ok" && <CheckCircle2 size={10} />}
          <span style={{ color: statusColor }}>{call.status}</span>
        </span>
      </div>
      {(argsStr || call.resultPreview) && (
        <div className="px-2 py-1 space-y-0.5">
          {argsStr && (
            <div className="text-[10.5px] text-[var(--color-dim)]">
              <span className="text-[var(--color-orange)]">args:</span>{" "}
              <span className="text-[var(--color-fg)] break-all">{argsStr}</span>
            </div>
          )}
          {call.resultPreview && (
            <div className="text-[10.5px] text-[var(--color-dim)] whitespace-pre-wrap break-words">
              <span className="text-[var(--color-lime)]">result:</span>{" "}
              <span className="text-[var(--color-fg)]">{call.resultPreview}</span>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

export function GhostBubble({ data }: { data: GhostBubbleData }) {
  return (
    <div className="flex gap-3 items-start py-2 px-1">
      {/* avatar */}
      <div
        className="w-6 h-6 shrink-0 flex items-center justify-center font-display font-bold text-[10px]"
        style={{
          background:
            "linear-gradient(135deg, var(--color-magenta), var(--color-cyan))",
          color: "var(--color-bg)",
          clipPath:
            "polygon(20% 0, 100% 0, 100% 80%, 80% 100%, 0 100%, 0 20%)",
        }}
      >
        ◆
      </div>

      <div className="flex-1 min-w-0">
        {/* header */}
        <div className="flex items-center gap-2 mb-1">
          <span
            className="font-display font-semibold text-[10px] tracking-hud"
            style={{ color: "var(--color-magenta)" }}
          >
            ◂ MAIN
          </span>
          <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight animate-pulse">
            {data.pending
              ? "esperando engine…"
              : data.done
              ? "finalizando…"
              : "streaming…"}
          </span>
        </div>

        <div
          className="font-mono text-[12.5px] leading-[1.55] text-[var(--color-fg)] break-words px-3 py-2 clip-hud-sm border"
          style={{
            background: "rgba(255,78,214,0.04)",
            borderColor: "rgba(255,78,214,0.20)",
          }}
        >
          {/* thinking */}
          {data.thinking && (
            <div
              className="mb-2 flex gap-2 italic text-[11.5px] text-[var(--color-dim)] leading-[1.5]"
              style={{
                paddingBottom: 6,
                borderBottom: "1px dashed var(--color-line)",
              }}
            >
              <Brain
                size={12}
                className="shrink-0 mt-0.5"
                style={{ color: "var(--color-dim)" }}
              />
              <div className="whitespace-pre-wrap break-words">
                {data.thinking}
              </div>
            </div>
          )}

          {/* tool cards */}
          {data.tools.length > 0 && (
            <div className="mb-1">
              {data.tools.map((t) => (
                <ToolCard key={t.id} call={t} />
              ))}
            </div>
          )}

          {/* live text — markdown */}
          {data.text ? (
            <ReactMarkdown remarkPlugins={[remarkGfm]} components={mdComponents}>
              {data.text}
            </ReactMarkdown>
          ) : !data.thinking && data.tools.length === 0 ? (
            <span className="text-[var(--color-dim)] italic">
              ◂ MAIN está pensando…
            </span>
          ) : null}

          {/* blinking cursor while streaming text */}
          {data.text && (
            <span
              className="inline-block w-1.5 h-3 ml-0.5 animate-pulse align-middle"
              style={{ background: "var(--color-magenta)" }}
            />
          )}
        </div>
      </div>
    </div>
  );
}
