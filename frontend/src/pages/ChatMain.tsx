import * as React from "react";
import {
  api,
  wsUrl,
  type AgentMessage,
  type MessageAttachmentRef,
} from "@/lib/api";
import { useWebSocket } from "@/lib/useWebSocket";
import { Composer } from "@/components/Composer";
import { MessageBubble } from "@/components/MessageBubble";
import {
  GhostBubble,
  type GhostBubbleData,
  type ToolCall,
} from "@/components/GhostBubble";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { StatusBar } from "@/components/StatusBar";

const POLL_MS = 2000;

/**
 * Backend Envelope shape (internal/ws/hub.go):
 *   { type: "message" | "stream", topic, payload, ts }
 *
 * For "message", payload is the persisted AgentMessage.
 * For "stream", payload contains live chunks of an in-flight turn.
 *
 * payload may arrive as object (json.RawMessage embedded) OR string (when
 * marshalled twice). We tolerate both.
 */
interface WsEnvelope {
  type: string;
  topic?: string;
  payload?: unknown;
  ts?: number;
}

interface WsAgentPayload {
  id: number;
  channel: string;
  direction: "in" | "out" | string;
  body: string;
  ts: number;
  is_read?: boolean;
}

type StreamKind = "text" | "tool_use" | "tool_result" | "thinking";

interface WsStreamPayload {
  kind: StreamKind;
  text?: string;
  tool_name?: string;
  tool_args?: unknown;
  tool_result?: string;
  session_id: string;
  /** monotonic seq within the turn — used to key tool cards */
  seq: number;
  /** true on the last chunk of a turn — when the persisted message is about to arrive */
  final?: boolean;
}

function parseEnvelopePayload<T>(payload: unknown): T | null {
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

function fromWs(m: WsAgentPayload): AgentMessage {
  return {
    id: m.id,
    channel: m.channel,
    direction: m.direction === "out" ? "out" : "in",
    body: m.body ?? "",
    ts: m.ts,
    isRead: !!m.is_read,
  };
}

export function ChatMain() {
  const [messages, setMessages] = React.useState<AgentMessage[]>([]);
  // ghost bubbles keyed by session_id — map allows multiple parallel turns
  const [ghosts, setGhosts] = React.useState<Record<string, GhostBubbleData>>(
    {}
  );
  const [error, setError] = React.useState<string | null>(null);
  const [pending, setPending] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const lastIdRef = React.useRef<number>(0);
  const pollingRef = React.useRef<number | null>(null);

  // ─── load helpers ──────────────────────────────
  const refresh = React.useCallback(async () => {
    try {
      const list = await api.listMessages();
      const lastId = list.length ? list[list.length - 1].id : 0;
      lastIdRef.current = lastId;
      setMessages(list);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error de red");
    }
  }, []);

  // initial fetch — always do this so we have history before WS opens
  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  // ─── polling fallback control ──────────────────
  const startPolling = React.useCallback(() => {
    if (pollingRef.current !== null) return;
    pollingRef.current = window.setInterval(refresh, POLL_MS);
  }, [refresh]);

  const stopPolling = React.useCallback(() => {
    if (pollingRef.current !== null) {
      window.clearInterval(pollingRef.current);
      pollingRef.current = null;
    }
  }, []);

  React.useEffect(() => {
    return () => stopPolling();
  }, [stopPolling]);

  // ─── stream handling ───────────────────────────
  const applyStreamChunk = React.useCallback((chunk: WsStreamPayload) => {
    const sid = chunk.session_id || "default";

    setGhosts((curr) => {
      // final=true → drop the ghost; the persisted "message" envelope
      // will arrive shortly with the consolidated body.
      if (chunk.final) {
        if (!(sid in curr)) return curr;
        const next = { ...curr };
        delete next[sid];
        return next;
      }

      const existing: GhostBubbleData = curr[sid] ?? {
        id: `stream-${sid}`,
        thinking: "",
        text: "",
        tools: [],
      };

      switch (chunk.kind) {
        case "text": {
          if (!chunk.text) return curr;
          return {
            ...curr,
            [sid]: { ...existing, text: existing.text + chunk.text },
          };
        }
        case "thinking": {
          if (!chunk.text) return curr;
          return {
            ...curr,
            [sid]: {
              ...existing,
              thinking: existing.thinking + chunk.text,
            },
          };
        }
        case "tool_use": {
          const id = `${sid}-${chunk.seq}`;
          const newCall: ToolCall = {
            id,
            name: chunk.tool_name ?? "tool",
            args: chunk.tool_args,
            status: "running",
          };
          // dedupe in case the same seq arrives twice
          const tools = existing.tools.some((t) => t.id === id)
            ? existing.tools
            : [...existing.tools, newCall];
          return {
            ...curr,
            [sid]: { ...existing, tools },
          };
        }
        case "tool_result": {
          // match the most recent running tool with same name (or last one)
          const tools = existing.tools.slice();
          // prefer last running
          let idx = -1;
          for (let i = tools.length - 1; i >= 0; i--) {
            if (
              tools[i].status === "running" &&
              (!chunk.tool_name || tools[i].name === chunk.tool_name)
            ) {
              idx = i;
              break;
            }
          }
          if (idx === -1 && tools.length > 0) idx = tools.length - 1;
          if (idx >= 0) {
            const preview = (chunk.tool_result ?? "").slice(0, 200);
            tools[idx] = {
              ...tools[idx],
              status: "ok",
              resultPreview: preview,
            };
          }
          return {
            ...curr,
            [sid]: { ...existing, tools },
          };
        }
        default:
          return curr;
      }
    });
  }, []);

  // ─── ws handler ────────────────────────────────
  const handleWsMessage = React.useCallback(
    (data: unknown) => {
      if (typeof data !== "object" || data === null) return;
      const evt = data as WsEnvelope;

      if (evt.type === "stream") {
        const chunk = parseEnvelopePayload<WsStreamPayload>(evt.payload);
        if (!chunk) return;
        applyStreamChunk(chunk);
        return;
      }

      if (evt.type !== "message") return;
      const msg = parseEnvelopePayload<WsAgentPayload>(evt.payload);
      if (!msg) return;
      const incoming = fromWs(msg);

      setMessages((curr) => {
        // merge by id; replace optimistic (negative id) bubbles whose body matches
        const idx = curr.findIndex((m) => m.id === incoming.id);
        if (idx >= 0) {
          const next = curr.slice();
          next[idx] = incoming;
          return next;
        }
        // try to reconcile an optimistic outgoing bubble (id < 0)
        if (incoming.direction === "in") {
          const optIdx = curr.findIndex(
            (m) => m.id < 0 && m.direction === "in" && m.body === incoming.body
          );
          if (optIdx >= 0) {
            const next = curr.slice();
            next[optIdx] = incoming;
            return next;
          }
        }
        const next = [...curr, incoming].sort((a, b) => a.ts - b.ts);
        return next;
      });
      lastIdRef.current = Math.max(lastIdRef.current, incoming.id);
      setError(null);
    },
    [applyStreamChunk]
  );

  const { status: wsStatus } = useWebSocket({
    url: wsUrl("/ws/agent"),
    onMessage: handleWsMessage,
    onFallback: () => {
      // ws is repeatedly failing — start polling so the UX keeps working
      startPolling();
    },
    onReconnected: () => {
      // ws came back — stop polling and resync once
      stopPolling();
      void refresh();
    },
  });

  // ─── auto-scroll on new messages or ghost activity ─
  const ghostList = React.useMemo(() => Object.values(ghosts), [ghosts]);
  const ghostSig = React.useMemo(
    () =>
      ghostList
        .map(
          (g) =>
            `${g.id}:${g.text.length}:${g.thinking.length}:${g.tools.length}:${g.tools.map((t) => t.status).join(",")}`
        )
        .join("|"),
    [ghostList]
  );

  React.useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages.length, pending, ghostSig]);

  // ─── send ──────────────────────────────────────
  async function handleSend(body: string, attachments: MessageAttachmentRef[]) {
    setPending(true);
    const optimisticId = -Date.now();
    setMessages((curr) => [
      ...curr,
      {
        id: optimisticId,
        channel: "web",
        direction: "in",
        body,
        ts: Math.floor(Date.now() / 1000),
        isRead: true,
      },
    ]);
    try {
      const res = await api.sendMessage(body, attachments);
      // reconcile optimistic bubble with the real id from the POST
      setMessages((curr) => {
        const next = curr.slice();
        const idx = next.findIndex((m) => m.id === optimisticId);
        if (idx >= 0) {
          next[idx] = { ...next[idx], id: res.id };
        }
        return next;
      });
      // when WS is the live channel, the agent reply will arrive over WS;
      // if we are in fallback or pre-open, do an explicit refresh.
      if (wsStatus !== "open") {
        await refresh();
      }
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "fallo enviando mensaje");
      setMessages((curr) => curr.filter((m) => m.id !== optimisticId));
    } finally {
      setPending(false);
    }
  }

  const isLive = wsStatus === "open";
  const transportLabel = (() => {
    switch (wsStatus) {
      case "open":
        return "ws · live";
      case "connecting":
        return "ws · connecting…";
      case "fallback":
        return `polling · ${POLL_MS / 1000}s`;
      case "closed":
        return "ws · reconnect…";
    }
  })();

  // when there's an active ghost bubble, we don't show the simple "pensando…" line
  const hasGhost = ghostList.length > 0;

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[
          { label: "AgentHub" },
          { label: "Chat / main-agent" },
        ]}
        status={
          error
            ? { label: "OFFLINE", tone: "danger" }
            : isLive
            ? { label: "LIVE", tone: "ok" }
            : { label: wsStatus === "fallback" ? "POLLING" : "CONNECTING", tone: "warn" }
        }
        right={
          <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
            {transportLabel}
          </span>
        }
      />

      <div className="flex-1 min-h-0 p-4 overflow-hidden">
        <HudPanel
          title="agente principal"
          sub={`session-aware · ${messages.length} mensajes`}
          accent="magenta"
          className="h-full"
        >
          <div
            ref={scrollRef}
            className="flex-1 min-h-0 overflow-y-auto pr-1"
          >
            {messages.length === 0 && !error && !hasGhost && (
              <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
                ▸ sin mensajes aún · escribí algo abajo
              </div>
            )}

            {messages.map((m) => (
              <MessageBubble
                key={m.id}
                message={m}
                topic={m.direction === "out" ? "main-agent" : null}
              />
            ))}

            {ghostList.map((g) => (
              <GhostBubble key={g.id} data={g} />
            ))}

            {pending && !hasGhost && (
              <div className="px-1 py-2 font-mono text-[10px] text-[var(--color-magenta)] tracking-hud animate-pulse">
                ◂ MAIN está pensando…
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
            <Composer onSend={handleSend} />
            <StatusBar transportLabel={transportLabel} />
          </div>
        </HudPanel>
      </div>
    </div>
  );
}
