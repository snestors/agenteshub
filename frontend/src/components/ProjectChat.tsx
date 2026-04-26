import * as React from "react";
import { api, type AgentMessage, type MessageAttachmentRef, type ProjectMessage } from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { Composer } from "@/components/Composer";
import { MessageBubble } from "@/components/MessageBubble";
import { GhostBubble, type GhostBubbleData, type ToolCall } from "@/components/GhostBubble";
import { StatusBar } from "@/components/StatusBar";

interface ProjectChatProps {
  projectId: number;
  sessionId: number;
  sessionName?: string;
}

interface WsEnvelope {
  type: string;
  topic?: string;
  payload?: unknown;
  ts?: number;
}

type StreamKind = "text" | "tool_use" | "tool_result" | "thinking" | "final" | "system";

interface WsStreamPayload {
  kind: StreamKind;
  text?: string;
  tool_name?: string;
  tool_args?: unknown;
  tool_result?: string;
  session_id?: string;
  seq: number;
  final?: boolean;
}

function parsePayload<T>(payload: unknown): T | null {
  if (!payload) return null;
  if (typeof payload === "string") {
    try {
      const obj = JSON.parse(payload);
      return obj && typeof obj === "object" ? (obj as T) : null;
    } catch {
      return null;
    }
  }
  if (typeof payload === "object") return payload as T;
  return null;
}

function projectMessageToAgent(m: ProjectMessage): AgentMessage {
  return {
    id: m.id,
    channel: m.channel || "project",
    direction: m.direction === "in" ? "in" : "out",
    body: m.body ?? "",
    ts: m.ts,
    isRead: true,
  };
}

export function ProjectChat({ projectId, sessionId, sessionName }: ProjectChatProps) {
  const topic = React.useMemo(() => `project_session:${sessionId}`, [sessionId]);
  const [messages, setMessages] = React.useState<AgentMessage[]>([]);
  const [ghosts, setGhosts] = React.useState<Record<string, GhostBubbleData>>({});
  const [pending, setPending] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const scrollRef = React.useRef<HTMLDivElement>(null);

  const refresh = React.useCallback(async () => {
    try {
      const rows = await api.listProjectMessages(projectId, sessionId);
      setMessages(rows.map(projectMessageToAgent));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando mensajes");
    }
  }, [projectId, sessionId]);

  React.useEffect(() => {
    setMessages([]);
    setGhosts({});
    setPending(false);
    void refresh();
  }, [refresh]);

  const applyStreamChunk = React.useCallback((chunk: WsStreamPayload) => {
    const sid = chunk.session_id || `project-${sessionId}`;
    setGhosts((curr) => {
      if (chunk.final) {
        const next = { ...curr };
        delete next[sid];
        delete next[`project-${sessionId}`];
        return next;
      }
      const existing: GhostBubbleData = curr[sid] ?? {
        id: `stream-${sid}`,
        thinking: "",
        text: "",
        tools: [],
      };
      switch (chunk.kind) {
        case "text":
          if (!chunk.text) return curr;
          return { ...curr, [sid]: { ...existing, text: existing.text + chunk.text } };
        case "thinking":
          if (!chunk.text) return curr;
          return {
            ...curr,
            [sid]: { ...existing, thinking: existing.thinking + chunk.text },
          };
        case "tool_use": {
          const id = `${sid}-${chunk.seq}`;
          const call: ToolCall = {
            id,
            name: chunk.tool_name ?? "tool",
            args: chunk.tool_args,
            status: "running",
          };
          const tools = existing.tools.some((t) => t.id === id)
            ? existing.tools
            : [...existing.tools, call];
          return { ...curr, [sid]: { ...existing, tools } };
        }
        case "tool_result": {
          const tools = existing.tools.slice();
          let idx = -1;
          for (let i = tools.length - 1; i >= 0; i--) {
            if (tools[i].status === "running") {
              idx = i;
              break;
            }
          }
          if (idx === -1 && tools.length > 0) idx = tools.length - 1;
          if (idx >= 0) {
            tools[idx] = {
              ...tools[idx],
              status: "ok",
              resultPreview: (chunk.tool_result ?? "").slice(0, 200),
            };
          }
          return { ...curr, [sid]: { ...existing, tools } };
        }
        default:
          return curr;
      }
    });
  }, [sessionId]);

  const handleTopic = React.useCallback(
    (_payload: unknown, env: WsEnvelope) => {
      if (env.type === "stream") {
        const chunk = parsePayload<WsStreamPayload>(env.payload);
        if (chunk) applyStreamChunk(chunk);
        return;
      }
      if (env.type !== "message") return;
      const msg = parsePayload<ProjectMessage>(env.payload);
      if (!msg) return;
      const incoming = projectMessageToAgent(msg);
      setMessages((curr) => {
        const idx = curr.findIndex((m) => m.id === incoming.id);
        if (idx >= 0) {
          const next = curr.slice();
          next[idx] = incoming;
          return next;
        }
        if (incoming.direction === "in") {
          const optIdx = curr.findIndex((m) => m.id < 0 && m.body === incoming.body);
          if (optIdx >= 0) {
            const next = curr.slice();
            next[optIdx] = incoming;
            return next;
          }
        }
        return [...curr, incoming].sort((a, b) => a.ts - b.ts || a.id - b.id);
      });
      setPending(false);
    },
    [applyStreamChunk]
  );

  const { status: wsStatus } = useTopic(topic, handleTopic, sessionId > 0);

  React.useEffect(() => {
    if (wsStatus === "open") void refresh();
  }, [wsStatus, refresh]);

  const ghostList = React.useMemo(() => Object.values(ghosts), [ghosts]);
  const ghostSig = React.useMemo(
    () => ghostList.map((g) => `${g.id}:${g.text.length}:${g.thinking.length}:${g.tools.length}`).join("|"),
    [ghostList]
  );

  React.useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages.length, ghostSig, pending]);

  async function handleSend(body: string, _attachments: MessageAttachmentRef[]) {
    const optimisticId = -Date.now();
    setPending(true);
    setMessages((curr) => [
      ...curr,
      {
        id: optimisticId,
        channel: "project",
        direction: "in",
        body,
        ts: Math.floor(Date.now() / 1000),
        isRead: true,
      },
    ]);
    try {
      await api.sendProjectMessage(projectId, sessionId, body);
      setError(null);
      window.setTimeout(() => void refresh(), 1200);
    } catch (err) {
      setError(err instanceof Error ? err.message : "fallo enviando mensaje");
      setMessages((curr) => curr.filter((m) => m.id !== optimisticId));
      setPending(false);
    }
  }

  const transportLabel =
    wsStatus === "open"
      ? "ws · live"
      : wsStatus === "connecting"
        ? "ws · connecting…"
        : "ws · reconnect…";
  const hasGhost = ghostList.length > 0;

  return (
    <div className="flex flex-col h-full min-h-0">
      <div
        ref={scrollRef}
        className="flex-1 min-h-0 overflow-y-auto pr-1"
      >
        {messages.length === 0 && !hasGhost && !error && (
          <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
            ▸ sesión {sessionName ?? sessionId} sin mensajes · escribí abajo
          </div>
        )}
        {messages.map((m) => (
          <MessageBubble key={m.id} message={m} />
        ))}
        {ghostList.map((g) => (
          <GhostBubble key={g.id} data={g} />
        ))}
        {pending && !hasGhost && (
          <div className="px-1 py-2 font-mono text-[10px] text-[var(--color-magenta)] tracking-hud animate-pulse">
            ◂ PROJECT SESSION está pensando…
          </div>
        )}
      </div>

      {error && (
        <div
          className="mt-2 px-3 py-2 font-mono text-[10px] clip-hud-sm"
          style={{
            background: "rgba(255, 92, 122, 0.08)",
            border: "1px solid rgba(255, 92, 122, 0.45)",
            color: "var(--color-danger)",
          }}
        >
          ✗ {error}
        </div>
      )}

      <div className="mt-2 -mx-4 -mb-3">
        <Composer onSend={handleSend} disabled={sessionId <= 0} />
        <StatusBar transportLabel={transportLabel} />
      </div>
    </div>
  );
}
