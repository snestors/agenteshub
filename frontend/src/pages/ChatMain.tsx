import * as React from "react";
import {
  api,
  type AgentMessage,
  type MessageAttachmentRef,
} from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { wsClient } from "@/lib/wsClient";
import { Composer } from "@/components/Composer";
import { MessageBubble } from "@/components/MessageBubble";
import { GhostBubble } from "@/components/GhostBubble";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { StatusBar } from "@/components/StatusBar";
import { useStreams } from "@/lib/streamsStore";

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
  engine?: string;
  model?: string;
}

interface SendMessageResult {
  id: number;
  message_id: number;
  accepted: boolean;
  engine: string;
  model: string;
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
    engine: m.engine,
    model: m.model,
  };
}

export function ChatMain() {
  // Live ghost bubbles for in-flight turns are managed by the global
  // StreamsProvider so they survive cross-navigation. We only read from it.
  const { agentGhostsList: ghostList } = useStreams();
  const [messages, setMessages] = React.useState<AgentMessage[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [pending, setPending] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const lastIdRef = React.useRef<number>(0);

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

  // ─── ws handler ────────────────────────────────
  // stream chunks are handled by StreamsProvider; here we only react to
  // 'message' envelopes (persisted history) to keep our list in sync.
  const handleWsMessage = React.useCallback(
    (_payload: unknown, evt: WsEnvelope) => {
      if (evt.type === "stream") return;
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
    []
  );

  // subscribe to the unified /ws endpoint, topic "agent"
  const { status: wsStatus } = useTopic("agent", handleWsMessage);

  // Keep the agent_status topic active for the shared connection; StatusBar
  // owns the payload rendering.
  useTopic("agent_status", () => {
    /* no-op */
  });

  // On reconnect, refresh once to reconcile anything missed while offline.
  React.useEffect(() => {
    if (wsStatus === "open") {
      void refresh();
    }
  }, [wsStatus, refresh]);

  // ─── auto-scroll on new messages or ghost activity ─
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
      const res = await wsClient.request<SendMessageResult>("send_message", {
        body,
        attachments,
      });
      // reconcile optimistic bubble with the real id from the WS ack
      setMessages((curr) => {
        const next = curr.slice();
        const idx = next.findIndex((m) => m.id === optimisticId);
        if (idx >= 0) {
          next[idx] = { ...next[idx], id: res.message_id ?? res.id };
        }
        return next;
      });
      // when WS is the live channel, the agent reply will arrive over WS;
      // If we are pre-open/reconnecting, do an explicit refresh after the ack path fails.
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
        return "ws · reconnect…";
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
            : { label: wsStatus === "connecting" ? "CONNECTING" : "RECONNECTING", tone: "warn" }
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
              <MessageBubble key={m.id} message={m} />
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
