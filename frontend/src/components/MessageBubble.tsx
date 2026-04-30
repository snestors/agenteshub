import * as React from "react";
import { cn } from "@/lib/utils";
import { api, type AgentMessage } from "@/lib/api";
import { MermaidBlock } from "@/components/MermaidBlock";
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
    const language = /language-(\w+)/.exec(className ?? "")?.[1];
    const inline = !className;
    if (!inline && language === "mermaid") {
      return <MermaidBlock content={String(children).replace(/\n$/, "")} />;
    }
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
          className,
        )}
        {...rest}
      >
        {children}
      </code>
    );
  },
  pre: ({ children }) => (
    <div
      className="my-2 px-3 py-2 overflow-x-auto clip-hud-sm font-mono text-[11.5px] leading-[1.5]"
      style={{
        background: "rgba(0,0,0,0.55)",
        border: "1px solid var(--color-line)",
        color: "var(--color-fg)",
      }}
    >
      {children}
    </div>
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
  const emptyBodyLabel =
    message.media_type === "image"
      ? "[imagen adjunta]"
      : message.media_type === "video"
        ? "[video adjunto]"
        : message.media_type === "audio"
          ? "[audio adjunto]"
          : message.media_type === "voice"
            ? "[nota de voz adjunta]"
            : message.media_type === "document"
              ? "[archivo adjunto]"
              : "[vacío]";
  const accent = isUser ? "var(--color-lime)" : "var(--color-magenta)";
  const role = isUser ? "USR" : "MAIN";
  const arrow = isUser ? "▸" : "◂";

  return (
    <div className="flex gap-2 items-start py-2 px-0 sm:gap-3 sm:px-1">
      {/* avatar — square 24px */}
      <div
        className="w-6 h-6 shrink-0 flex items-center justify-center font-display font-bold text-[10px]"
        style={{
          background: isUser
            ? "linear-gradient(135deg, var(--color-lime), var(--color-cyan))"
            : "linear-gradient(135deg, var(--color-magenta), var(--color-cyan))",
          color: "var(--color-bg)",
          clipPath: "polygon(20% 0, 100% 0, 100% 80%, 80% 100%, 0 100%, 0 20%)",
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
            "px-2.5 py-2 clip-hud-sm border sm:px-3",
            isUser
              ? "bg-[rgba(163,255,78,0.04)] border-[rgba(163,255,78,0.20)]"
              : "bg-[rgba(255,78,214,0.04)] border-[rgba(255,78,214,0.20)]",
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
            <span
              className={cn(
                "italic",
                !isUser && !message.media_type
                  ? "font-semibold text-[var(--color-danger)]"
                  : "text-[var(--color-dim)]",
              )}
            >
              {!isUser && !message.media_type
                ? "⚠ El engine no devolvió respuesta. Probá cambiar de modelo o reintentar."
                : emptyBodyLabel}
            </span>
          )}
        </div>

        {message.media_type === "image" && message.media_path && (
          <ImagePreview
            src={api.fileUrl(message.media_path)}
            caption={message.media_caption || message.body}
          />
        )}

        {message.media_type === "video" && message.media_path && (
          <VideoPreview
            src={api.fileUrl(message.media_path)}
            caption={message.media_caption || message.body}
          />
        )}

        {/* topic-pill (placeholder, only on agent replies) */}
        {!isUser && topic && (
          <div className="mt-1.5">
            <span className="topic-pill">▸ {topic}</span>
          </div>
        )}

        {/* activity audit (collapsed by default) — assistant only */}
        {!isUser && message.activity && (
          <ActivityPanel activity={message.activity} />
        )}
      </div>
    </div>
  );
}

function ActivityPanel({
  activity,
}: {
  activity: NonNullable<AgentMessage["activity"]>;
}) {
  const [open, setOpen] = React.useState(false);
  const tools = activity.tools ?? [];
  const hasThinking = !!activity.thinking;
  const summary = [
    tools.length > 0
      ? `${tools.length} ${tools.length === 1 ? "tool" : "tools"}`
      : "",
    hasThinking ? "thinking" : "",
  ]
    .filter(Boolean)
    .join(" · ");
  if (!summary) return null;

  return (
    <div className="mt-1.5">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)] hover:text-[var(--color-cyan)] transition-colors cursor-pointer flex items-center gap-1"
      >
        <span>{open ? "▾" : "▸"}</span>
        <span>actividad · {summary}</span>
      </button>
      {open && (
        <div
          className="mt-1.5 px-3 py-2 clip-hud-sm border font-mono text-[11px] leading-[1.55] text-[var(--color-dim)] space-y-2"
          style={{
            background: "rgba(94, 240, 255, 0.03)",
            borderColor: "rgba(94, 240, 255, 0.15)",
          }}
        >
          {hasThinking && (
            <div>
              <div className="text-[9px] uppercase tracking-hud-tight text-[var(--color-cyan)] mb-1">
                ▸ thinking
              </div>
              <div className="italic whitespace-pre-wrap break-words">
                {activity.thinking}
              </div>
            </div>
          )}
          {tools.length > 0 && (
            <div>
              <div className="text-[9px] uppercase tracking-hud-tight text-[var(--color-orange)] mb-1">
                ▸ tools usadas
              </div>
              <ol className="space-y-1.5 list-none">
                {tools.map((t, i) => (
                  <li
                    key={t.id ?? i}
                    className="border-l-2 pl-2"
                    style={{ borderColor: "var(--color-line)" }}
                  >
                    <div className="text-[var(--color-fg)]">
                      <span style={{ color: "var(--color-cyan)" }}>
                        {t.name}
                      </span>
                      <span className="text-[var(--color-dim)] ml-2">
                        [{t.status}]
                      </span>
                    </div>
                    {t.result_preview && (
                      <div className="mt-0.5 italic break-words text-[10.5px]">
                        {t.result_preview}
                      </div>
                    )}
                  </li>
                ))}
              </ol>
            </div>
          )}
        </div>
      )}
    </div>
  );
}

function ImagePreview({ src, caption }: { src: string; caption?: string }) {
  const [open, setOpen] = React.useState(false);
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="mt-2 block max-w-[360px] cursor-zoom-in text-left"
        title="Click para ampliar"
      >
        <img
          src={src}
          alt={caption || "imagen adjunta"}
          className="max-h-[260px] max-w-full rounded border object-contain"
          style={{
            borderColor: "rgba(94,240,255,0.24)",
            background: "rgba(0,0,0,0.35)",
          }}
          loading="lazy"
        />
      </button>
      {open && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/85 p-6 cursor-zoom-out"
          onClick={() => setOpen(false)}
        >
          <img
            src={src}
            alt={caption || "imagen adjunta"}
            className="max-h-full max-w-full object-contain rounded border"
            style={{ borderColor: "rgba(94,240,255,0.40)" }}
          />
        </div>
      )}
    </>
  );
}

function VideoPreview({ src, caption }: { src: string; caption?: string }) {
  return (
    <div className="mt-2 max-w-[420px]">
      <video
        src={src}
        controls
        preload="metadata"
        className="max-h-[300px] w-full rounded border bg-black"
        style={{ borderColor: "rgba(94,240,255,0.24)" }}
      />
      {caption ? (
        <div className="mt-1 text-[10.5px] text-[var(--color-dim)] break-words">
          {caption}
        </div>
      ) : null}
    </div>
  );
}
