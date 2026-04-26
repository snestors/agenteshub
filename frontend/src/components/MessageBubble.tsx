import { cn } from "@/lib/utils";
import type { AgentMessage } from "@/lib/api";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

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

// HUD-flavored markdown renderers — keep cyberpunk palette consistent.
const mdComponents: Components = {
  p: ({ children }) => (
    <p className="my-1.5 first:mt-0 last:mb-0 leading-[1.55]">{children}</p>
  ),
  a: ({ children, href }) => (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className="text-[var(--color-cyan)] hover:underline"
    >
      {children}
    </a>
  ),
  strong: ({ children }) => (
    <strong className="font-semibold text-[var(--color-fg)]">{children}</strong>
  ),
  em: ({ children }) => (
    <em className="italic text-[var(--color-fg)]">{children}</em>
  ),
  h1: ({ children }) => (
    <h1
      className="font-display font-bold text-[15px] tracking-hud my-2"
      style={{ color: "var(--color-fg)" }}
    >
      {children}
    </h1>
  ),
  h2: ({ children }) => (
    <h2
      className="font-display font-semibold text-[14px] tracking-hud my-2"
      style={{ color: "var(--color-fg)" }}
    >
      {children}
    </h2>
  ),
  h3: ({ children }) => (
    <h3
      className="font-display font-semibold text-[13px] tracking-hud-tight my-1.5"
      style={{ color: "var(--color-fg)" }}
    >
      {children}
    </h3>
  ),
  ul: ({ children }) => (
    <ul className="list-disc pl-4 my-1.5 marker:text-[var(--color-magenta)]">
      {children}
    </ul>
  ),
  ol: ({ children }) => (
    <ol className="list-decimal pl-4 my-1.5 marker:text-[var(--color-magenta)]">
      {children}
    </ol>
  ),
  li: ({ children }) => <li className="my-0.5">{children}</li>,
  blockquote: ({ children }) => (
    <blockquote
      className="my-2 pl-3 italic text-[var(--color-dim)]"
      style={{ borderLeft: "2px solid var(--color-magenta)" }}
    >
      {children}
    </blockquote>
  ),
  hr: () => (
    <hr
      className="my-3 border-0"
      style={{ borderTop: "1px solid var(--color-line)" }}
    />
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
      <code
        className={cn(
          "font-mono text-[11.5px] block whitespace-pre",
          className
        )}
        {...rest}
      >
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
  table: ({ children }) => (
    <div className="my-2 overflow-x-auto">
      <table
        className="w-full text-[11.5px] border-collapse"
        style={{ border: "1px solid var(--color-line)" }}
      >
        {children}
      </table>
    </div>
  ),
  thead: ({ children }) => (
    <thead style={{ background: "rgba(94, 240, 255, 0.06)" }}>{children}</thead>
  ),
  th: ({ children }) => (
    <th
      className="font-semibold text-left px-2 py-1"
      style={{
        border: "1px solid var(--color-line)",
        color: "var(--color-fg)",
      }}
    >
      {children}
    </th>
  ),
  td: ({ children }) => (
    <td
      className="px-2 py-1 align-top"
      style={{ border: "1px solid var(--color-line)" }}
    >
      {children}
    </td>
  ),
};

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
        {/* header: role · time · channel · engine·model (assistant) */}
        <div className="flex items-center gap-2 mb-1 flex-wrap">
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
          {!isUser && (message.engine || message.model) && (
            <span
              className="font-mono text-[9px] tracking-hud-tight px-1.5 py-px clip-tag"
              style={{
                color: "var(--color-cyan)",
                background: "rgba(94, 240, 255, 0.08)",
                border: "1px solid rgba(94, 240, 255, 0.30)",
              }}
              title="engine · model que respondió"
            >
              {[message.engine, message.model].filter(Boolean).join(" · ")}
            </span>
          )}
        </div>

        {/* body */}
        <div
          className={cn(
            "font-mono text-[12.5px] leading-[1.55] text-[var(--color-fg)] break-words",
            "px-3 py-2 clip-hud-sm border",
            isUser
              ? "bg-[rgba(163,255,78,0.04)] border-[rgba(163,255,78,0.20)]"
              : "bg-[rgba(255,78,214,0.04)] border-[rgba(255,78,214,0.20)]"
          )}
        >
          {message.body ? (
            <ReactMarkdown
              remarkPlugins={[remarkGfm]}
              components={mdComponents}
            >
              {message.body}
            </ReactMarkdown>
          ) : (
            <span className="text-[var(--color-dim)] italic">[vacío]</span>
          )}
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
