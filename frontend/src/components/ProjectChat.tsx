import * as React from "react";
import { api, DEFAULT_REASONING_EFFORTS, FALLBACK_ENGINES, type AgentMessage, type ConversationRuntime, type EngineDef, type MessageAttachmentRef, type ProjectMessage, type ProjectSession, type RuntimeToolState } from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { Composer } from "@/components/Composer";
import { MessageBubble } from "@/components/MessageBubble";
import { GhostBubble, type GhostBubbleData, type SubagentStats, type ToolCall } from "@/components/GhostBubble";

function extractStatsFromMeta(meta: Record<string, unknown> | undefined): SubagentStats {
  if (!meta || typeof meta !== "object") return {};
  const num = (v: unknown): number | undefined =>
    typeof v === "number" && Number.isFinite(v) ? v : undefined;
  const str = (v: unknown): string | undefined =>
    typeof v === "string" && v.length > 0 ? v : undefined;
  return {
    agentId: str(meta.agent_id),
    agentType: str(meta.agent_type),
    status: str(meta.status),
    totalDurationMs: num(meta.total_duration_ms),
    totalTokens: num(meta.total_tokens),
    totalToolUseCount: num(meta.total_tool_use_count),
    toolStats:
      meta.tool_stats && typeof meta.tool_stats === "object"
        ? (meta.tool_stats as Record<string, unknown>)
        : undefined,
  };
}

interface ProjectChatProps {
  projectId: number;
  sessionId: number;
  sessionName?: string;
  engine?: string;
  model?: string;
  reasoningEffort?: string;
  sessions?: ProjectSession[];
  onSessionSelect?: (sessionId: number) => void;
  onCreateSession?: () => void;
  onDeleteSession?: (sessionId: number) => void;
  onSessionConfigChange?: (patch: Partial<ProjectSession>) => void;
}

interface WsEnvelope {
  type: string;
  topic?: string;
  payload?: unknown;
  ts?: number;
}

type StreamKind =
  | "text"
  | "tool_use"
  | "tool_result"
  | "thinking"
  | "final"
  | "system"
  | "subagent_stats";

interface WsStreamPayload {
  kind: StreamKind;
  text?: string;
  tool_name?: string;
  tool_use_id?: string;
  tool_args?: unknown;
  tool_result?: string;
  meta?: Record<string, unknown>;
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
    activity: m.activity,
    ts: m.ts,
    isRead: true,
  };
}

function runtimeToGhost(run: ConversationRuntime): GhostBubbleData | null {
  if (!run.session_id) return null;
  // Only hydrate ghosts for runs that are actually live. A finished run is
  // already covered by the persisted session_message; hydrating it keeps the
  // composer disabled and surfaces a stuck "finalizando…"/"pensando…" label
  // forever (e.g. when the daemon was restarted mid-turn so the final WS
  // chunk that would clear the ghost never arrives).
  if (run.status !== "running") return null;
  const tools: ToolCall[] = (run.tools ?? []).map((t: RuntimeToolState, idx) => ({
    id: t.id || `${run.session_id}-${idx}`,
    name: t.name,
    args: t.args,
    status: t.status === "cancelled" ? "error" : t.status,
    resultPreview: t.result_preview,
    startedAt: t.started_at,
    finishedAt: t.finished_at,
    subagentStats: extractStatsFromMeta(t.subagent_stats),
    claudeToolUseID: t.id,
  }));
  return {
    id: `stream-${run.session_id}`,
    thinking: run.thinking ?? "",
    text: run.text ?? "",
    tools,
    done: run.status !== "running",
    pending: run.status === "running" && !(run.text || run.thinking || tools.length > 0),
  };
}

export function ProjectChat({
  projectId,
  sessionId,
  sessionName,
  engine,
  model,
  reasoningEffort,
  sessions = [],
  onSessionSelect,
  onCreateSession,
  onDeleteSession,
  onSessionConfigChange,
}: ProjectChatProps) {
  const topic = React.useMemo(() => `project_session:${sessionId}`, [sessionId]);
  const [messages, setMessages] = React.useState<AgentMessage[]>([]);
  const [ghosts, setGhosts] = React.useState<Record<string, GhostBubbleData>>({});
  const [pending, setPending] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [modelChanging, setModelChanging] = React.useState(false);
  const scrollRef = React.useRef<HTMLDivElement>(null);

  React.useEffect(() => {
    void api.listEngines().then((list) => {
      if (list.length > 0) setEngines(list);
    }).catch(() => undefined);
  }, []);
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
    // Check if a turn is already in flight (e.g. user navigated away and back).
    if (sessionId > 0) {
      api.getProjectRunStatus(projectId, sessionId)
        .then(({ running }) => { if (running) setPending(true); })
        .catch(() => {});
      api.getProjectRuntime(projectId, sessionId)
        .then((run) => {
          if (!run) return;
          if (run.status === "running") setPending(true);
          const ghost = runtimeToGhost(run);
          if (!ghost || !run.session_id) return;
          setGhosts({ [run.session_id]: ghost });
        })
        .catch(() => {});
    }
  }, [refresh, projectId, sessionId]);

  const applyStreamChunk = React.useCallback((chunk: WsStreamPayload) => {
    const sid = chunk.session_id || `project-${sessionId}`;
    if (chunk.final) {
      setPending(false);
    }
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
            startedAt: Date.now(),
            claudeToolUseID: chunk.tool_use_id,
          };
          const tools = existing.tools.some((t) => t.id === id)
            ? existing.tools
            : [...existing.tools, call];
          return { ...curr, [sid]: { ...existing, tools } };
        }
        case "subagent_stats": {
          const targetID = chunk.tool_use_id;
          if (!targetID) return curr;
          const tools = existing.tools.map((t) =>
            t.claudeToolUseID === targetID
              ? { ...t, subagentStats: extractStatsFromMeta(chunk.meta) }
              : t,
          );
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
              finishedAt: Date.now(),
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
    } catch (err) {
      setError(err instanceof Error ? err.message : "fallo enviando mensaje");
      setMessages((curr) => curr.filter((m) => m.id !== optimisticId));
      setPending(false);
    }
  }

  async function handleCancel() {
    try {
      await api.cancelProjectRun(projectId, sessionId);
    } catch {
      // 409 "no turn running" is common when the backend already finished
      // but the UI still has a stale ghost. Fall through and reset state
      // anyway — leaving the user trapped is worse than an idempotent reset.
    }
    setPending(false);
    setGhosts({});
    await refresh();
  }

  const transportLabel =
    wsStatus === "open"
      ? "ws · live"
      : wsStatus === "connecting"
        ? "ws · connecting…"
        : "ws · reconnect…";
  const hasGhost = ghostList.length > 0;
  const isRunning = pending || hasGhost;
  const engineDef = engines.find((e) => e.engine === engine) ?? FALLBACK_ENGINES.find((e) => e.engine === engine) ?? FALLBACK_ENGINES[0];
  const modelOptions = engineDef?.models ?? [];
  const effortOptions = engineDef?.reasoning_efforts?.length ? engineDef.reasoning_efforts : DEFAULT_REASONING_EFFORTS;
  const selectedModel = model || modelOptions[0] || "";
  const selectedEffort = reasoningEffort || (effortOptions.includes("medium") ? "medium" : effortOptions[0] ?? "");

  async function applySessionModel(nextModel: string, nextEffort: string) {
    if (!nextModel) return;
    setModelChanging(true);
    try {
      const res = await api.setProjectSessionModel(projectId, sessionId, {
        model: nextModel,
        reasoning_effort: nextEffort,
      });
      onSessionConfigChange?.({ model: res.model, reasoning_effort: res.reasoning_effort });
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "fallo cambiando modelo");
    } finally {
      setModelChanging(false);
    }
  }

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
        <Composer onSend={handleSend} disabled={sessionId <= 0 || isRunning} />
        {/* Session status bar — reemplaza StatusBar del main agent */}
        <div
          className="flex items-center gap-3 px-4 py-1.5 font-mono text-[10px] tracking-hud-tight border-t border-[var(--color-line)] select-none"
          style={{ background: "rgba(0,0,0,0.55)", minHeight: 26 }}
        >
          <select
            value={sessionId}
            onChange={(e) => onSessionSelect?.(Number(e.target.value))}
            className="clip-tag bg-transparent outline-none cursor-pointer max-w-[180px]"
            style={{
              color: "var(--color-magenta)",
              border: "1px solid rgba(255,78,214,0.45)",
              background: "rgba(255,78,214,0.10)",
              padding: "1px 6px",
              font: "inherit",
              letterSpacing: "inherit",
            }}
            title="cambiar sesión"
          >
            {(sessions.length > 0 ? sessions : [{ id: sessionId, name: sessionName ?? String(sessionId) } as ProjectSession]).map((s) => (
              <option key={s.id} value={s.id} style={{ background: "#0a0f24" }}>{s.name}</option>
            ))}
          </select>
          <button
            type="button"
            onClick={onCreateSession}
            className="clip-tag cursor-pointer hover:opacity-80"
            style={{
              color: "var(--color-lime)",
              border: "1px solid rgba(163,255,78,0.55)",
              background: "rgba(163,255,78,0.10)",
              font: "inherit",
              letterSpacing: "inherit",
              padding: "1px 6px",
            }}
            title="crear sesión"
          >
            +
          </button>
          {onDeleteSession && (
            <button
              type="button"
              onClick={() => onDeleteSession(sessionId)}
              disabled={isRunning}
              className="clip-tag cursor-pointer hover:opacity-80 disabled:opacity-40 disabled:cursor-not-allowed"
              style={{
                color: "var(--color-danger)",
                border: "1px solid rgba(255,92,122,0.45)",
                background: "rgba(255,92,122,0.08)",
                font: "inherit",
                letterSpacing: "inherit",
                padding: "1px 6px",
              }}
              title="eliminar sesión actual"
            >
              ×
            </button>
          )}

          <select
            value={selectedModel}
            disabled={isRunning || modelChanging}
            onChange={(e) => void applySessionModel(e.target.value, selectedEffort)}
            className="clip-tag bg-transparent outline-none cursor-pointer"
            style={{
              color: modelChanging ? "var(--color-dim)" : "var(--color-lime)",
              border: "1px solid rgba(163,255,78,0.45)",
              background: "rgba(163,255,78,0.10)",
              padding: "1px 6px",
              font: "inherit",
              letterSpacing: "inherit",
            }}
            title={`modelo para esta sesión ${engine ? `(${engine})` : ""}`}
          >
            {modelOptions.map((m) => (
              <option key={m} value={m} style={{ background: "#0a0f24" }}>{m}</option>
            ))}
          </select>

          <select
            value={selectedEffort}
            disabled={isRunning || modelChanging}
            onChange={(e) => void applySessionModel(selectedModel, e.target.value)}
            className="clip-tag bg-transparent outline-none cursor-pointer"
            style={{
              color: modelChanging ? "var(--color-dim)" : "var(--color-orange)",
              border: "1px solid rgba(255,159,67,0.45)",
              background: "rgba(255,159,67,0.10)",
              padding: "1px 6px",
              font: "inherit",
              letterSpacing: "inherit",
            }}
            title="reasoning effort para esta sesión"
          >
            {effortOptions.map((eff) => (
              <option key={eff} value={eff} style={{ background: "#0a0f24" }}>{eff}</option>
            ))}
          </select>

          {isRunning && (
            <>
              <span className="text-[var(--color-dim)]">·</span>
              <button
                onClick={() => void handleCancel()}
                className="clip-tag cursor-pointer hover:opacity-80"
                style={{
                  background: "rgba(255,92,122,0.08)",
                  border: "1px solid rgba(255,92,122,0.4)",
                  color: "var(--color-danger)",
                  font: "inherit",
                  letterSpacing: "inherit",
                  padding: "1px 6px",
                }}
              >
                ✕ cancelar
              </button>
            </>
          )}

          <span className="ml-auto text-[var(--color-dim)]">{transportLabel}</span>
        </div>
      </div>
    </div>
  );
}
