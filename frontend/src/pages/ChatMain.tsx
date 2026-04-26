import * as React from "react";
import { api, type AgentMessage } from "@/lib/api";
import { Composer } from "@/components/Composer";
import { MessageBubble } from "@/components/MessageBubble";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";

const POLL_MS = 2000;

export function ChatMain() {
  const [messages, setMessages] = React.useState<AgentMessage[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [pending, setPending] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);
  const lastIdRef = React.useRef<number>(0);

  // load + poll
  React.useEffect(() => {
    let alive = true;

    async function refresh() {
      try {
        const list = await api.listMessages();
        if (!alive) return;
        // only setState when last id changed (avoid useless re-renders)
        const lastId = list.length ? list[list.length - 1].id : 0;
        if (lastId !== lastIdRef.current || list.length !== messages.length) {
          lastIdRef.current = lastId;
          setMessages(list);
        }
        setError(null);
      } catch (err) {
        if (!alive) return;
        setError(err instanceof Error ? err.message : "error de red");
      }
    }

    void refresh();
    const id = window.setInterval(refresh, POLL_MS);
    return () => {
      alive = false;
      window.clearInterval(id);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // auto-scroll on new messages
  React.useEffect(() => {
    const el = scrollRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [messages.length, pending]);

  async function handleSend(body: string) {
    setPending(true);
    // optimistic: append a fake "sending" entry
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
      await api.sendMessage(body);
      // refresh immediately to pick up the agent reply
      const list = await api.listMessages();
      lastIdRef.current = list.length ? list[list.length - 1].id : 0;
      setMessages(list);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "fallo enviando mensaje");
      // remove the optimistic bubble
      setMessages((curr) => curr.filter((m) => m.id !== optimisticId));
    } finally {
      setPending(false);
    }
  }

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
            : { label: "ONLINE", tone: "ok" }
        }
        right={
          <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
            polling · {POLL_MS / 1000}s
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
          {/* scroll container */}
          <div
            ref={scrollRef}
            className="flex-1 min-h-0 overflow-y-auto pr-1"
          >
            {messages.length === 0 && !error && (
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

            {pending && (
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
          </div>
        </HudPanel>
      </div>
    </div>
  );
}
