import * as React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Bot, CalendarClock } from "lucide-react";
import {
  api,
  FALLBACK_ENGINES,
  type AgentRun,
  type AgentSchedule,
  type EngineDef,
  type MiniAgent,
} from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { GhostBubble, type GhostBubbleData, type ToolCall } from "@/components/GhostBubble";

type Tab = "schedules" | "runs" | "run";

function rel(ts?: number): string {
  if (!ts) return "—";
  const d = ts - Math.floor(Date.now() / 1000);
  const abs = Math.abs(d);
  const unit = abs < 3600 ? `${Math.max(1, Math.floor(abs / 60))}m` : abs < 86400 ? `${Math.floor(abs / 3600)}h` : `${Math.floor(abs / 86400)}d`;
  return d >= 0 ? `en ${unit}` : `hace ${unit}`;
}

export function Agents() {
  const params = useParams();
  const id = params.id ? Number(params.id) : 0;
  return id > 0 ? <AgentDetail id={id} /> : <AgentList />;
}

function AgentList() {
  const nav = useNavigate();
  const [agents, setAgents] = React.useState<MiniAgent[]>([]);
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [open, setOpen] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      setAgents(await api.listAgents());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando agentes");
    }
  }, []);

  React.useEffect(() => {
    void refresh();
    void api.listEngines().then(setEngines).catch(() => undefined);
  }, [refresh]);

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Mini-agentes" }]}
        status={error ? { label: "ERROR", tone: "danger" } : { label: "READY", tone: "ok" }}
        right={<HudButton onClick={() => setOpen(true)} label="+ nuevo agente" accent="var(--color-orange)" />}
      />
      <div className="flex-1 min-h-0 p-4 overflow-y-auto">
        {error && <ErrorBox msg={error} />}
        <div className="grid grid-cols-3 gap-4">
          {agents.map((a) => {
            const tone = a.enabled ? "var(--color-lime)" : "var(--color-danger)";
            return (
              <button key={a.id} onClick={() => nav(`/agents/${a.id}`)} className="text-left cursor-pointer">
                <HudPanel accent={a.enabled ? "orange" : "danger"} className="min-h-[190px] hover:opacity-90 transition-opacity">
                  <div className="flex items-start justify-between">
                    <Bot size={22} style={{ color: "var(--color-orange)" }} />
                    <span className="px-2 py-0.5 clip-tag font-mono text-[9px] tracking-hud" style={{ color: tone, border: `1px solid ${tone}` }}>
                      {a.enabled ? "enabled" : "paused"}
                    </span>
                  </div>
                  <div className="mt-3 font-display text-[18px] font-bold tracking-hud text-[var(--color-fg)]">{a.name}</div>
                  <div className="mt-2 font-mono text-[11px] text-[var(--color-dim)] line-clamp-3">{a.description || "sin descripción"}</div>
                  <div className="mt-auto pt-3 grid grid-cols-3 gap-2 font-mono text-[9px]">
                    <MiniStat label="engine" value={a.engine} />
                    <MiniStat label="next" value={a.next_run ? rel(a.next_run) : "—"} />
                    <MiniStat label="24h" value={String(a.runs_24h ?? 0)} />
                  </div>
                </HudPanel>
              </button>
            );
          })}
        </div>
        {agents.length === 0 && !error && (
          <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
            ▸ sin mini-agentes · creá el primero
          </div>
        )}
      </div>
      {open && <AgentModal engines={engines} onClose={() => setOpen(false)} onCreated={(a) => nav(`/agents/${a.id}`)} />}
    </div>
  );
}

function AgentDetail({ id }: { id: number }) {
  const nav = useNavigate();
  const [agent, setAgent] = React.useState<MiniAgent | null>(null);
  const [schedules, setSchedules] = React.useState<AgentSchedule[]>([]);
  const [runs, setRuns] = React.useState<AgentRun[]>([]);
  const [tab, setTab] = React.useState<Tab>("schedules");
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      const res = await api.getAgent(id);
      setAgent(res.agent);
      setSchedules(res.schedules);
      setRuns(await api.listAgentRuns(id, 50));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando agente");
    }
  }, [id]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  async function toggleAgent() {
    if (!agent) return;
    await api.setAgentEnabled(agent.id, !agent.enabled);
    await refresh();
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Mini-agentes" }, { label: agent?.name ?? String(id) }]}
        status={error ? { label: "ERROR", tone: "danger" } : { label: agent?.enabled ? "ENABLED" : "PAUSED", tone: agent?.enabled ? "ok" : "warn" }}
        right={
          <div className="flex items-center gap-3">
            <HudButton onClick={() => void toggleAgent()} label={agent?.enabled ? "pausar" : "reanudar"} accent={agent?.enabled ? "var(--color-danger)" : "var(--color-lime)"} />
            <button onClick={() => nav("/agents")} className="font-mono text-[10px] text-[var(--color-dim)] hover:text-[var(--color-cyan)] cursor-pointer">← lista</button>
          </div>
        }
      />
      <div className="flex-1 min-h-0 p-4">
        <HudPanel title={agent?.name ?? "mini-agente"} sub={agent ? `${agent.engine} · ${schedules.length} schedules` : "loading"} accent="orange">
          {error && <ErrorBox msg={error} />}
          <div className="flex gap-2 mb-3">
            {(["schedules", "runs", "run"] as Tab[]).map((t) => (
              <button
                key={t}
                onClick={() => setTab(t)}
                className="px-3 py-1 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer"
                style={{
                  color: tab === t ? "var(--color-orange)" : "var(--color-dim)",
                  border: `1px solid ${tab === t ? "var(--color-orange)" : "var(--color-line)"}`,
                  background: tab === t ? "rgba(255,159,67,0.10)" : "transparent",
                }}
              >
                {t === "run" ? "run now" : t}
              </button>
            ))}
          </div>
          {agent && tab === "schedules" && <SchedulesTab agent={agent} schedules={schedules} onChanged={refresh} />}
          {agent && tab === "runs" && <RunsTab runs={runs} onRefresh={refresh} />}
          {agent && tab === "run" && <RunNowTab agent={agent} onDone={refresh} />}
        </HudPanel>
      </div>
    </div>
  );
}

function SchedulesTab({ agent, schedules, onChanged }: { agent: MiniAgent; schedules: AgentSchedule[]; onChanged: () => Promise<void> }) {
  const [cron, setCron] = React.useState("*/5 * * * *");
  const [prompt, setPrompt] = React.useState("");
  const [target, setTarget] = React.useState("main-agent");
  const [error, setError] = React.useState<string | null>(null);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    try {
      await api.addAgentSchedule(agent.id, { cron_expr: cron, prompt_template: prompt, notify_target: target });
      setPrompt("");
      setError(null);
      await onChanged();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error agregando schedule");
    }
  }

  return (
    <div className="flex-1 min-h-0 grid grid-cols-[1fr_360px] gap-4">
      <div className="overflow-y-auto pr-1">
        {schedules.map((s) => (
          <div key={s.id} className="mb-2 px-3 py-2 clip-hud-sm font-mono text-[10px]" style={{ border: "1px solid var(--color-line)", background: "rgba(255,255,255,0.03)" }}>
            <div className="flex items-center justify-between gap-2">
              <span className="text-[var(--color-fg)]">{s.cron_expr}</span>
              <span className={s.enabled ? "text-[var(--color-lime)]" : "text-[var(--color-danger)]"}>{s.enabled ? "enabled" : "paused"}</span>
            </div>
            <div className="mt-1 text-[var(--color-dim)] line-clamp-2">{s.prompt_template}</div>
            <div className="mt-2 flex items-center justify-between text-[9px]">
              <span className="text-[var(--color-cyan)]"><CalendarClock size={10} className="inline mr-1" /> next {rel(s.next_run)}</span>
              <span className="text-[var(--color-magenta)]">{s.notify_target}</span>
              <div className="flex gap-2">
                <button onClick={() => api.setAgentScheduleEnabled(agent.id, s.id, !s.enabled).then(onChanged)} className="text-[var(--color-orange)] cursor-pointer">{s.enabled ? "pausar" : "activar"}</button>
                <button onClick={() => api.deleteAgentSchedule(agent.id, s.id).then(onChanged)} className="text-[var(--color-danger)] cursor-pointer">delete</button>
              </div>
            </div>
          </div>
        ))}
        {schedules.length === 0 && <Empty label="sin schedules" />}
      </div>
      <form onSubmit={add} className="flex flex-col">
        <Field label="cron expr" value={cron} onChange={setCron} />
        <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">prompt_template</label>
        <textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} className="mt-1 min-h-[150px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} />
        <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">notify_target</label>
        <select value={target} onChange={(e) => setTarget(e.target.value)} className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }}>
          <option value="main-agent">main-agent</option>
          <option value="none">none</option>
          <option value="topic:general">topic:general</option>
        </select>
        {error && <ErrorBox msg={error} />}
        <HudButton type="submit" label="+ agregar schedule" accent="var(--color-orange)" className="mt-4 self-end" />
      </form>
    </div>
  );
}

function RunsTab({ runs, onRefresh }: { runs: AgentRun[]; onRefresh: () => Promise<void> }) {
  const [open, setOpen] = React.useState<number | null>(null);
  return (
    <div className="flex-1 min-h-0 overflow-y-auto pr-1">
      <div className="flex justify-end mb-2">
        <HudButton onClick={() => void onRefresh()} label="refresh" accent="var(--color-cyan)" />
      </div>
      {runs.map((r) => (
        <div key={r.id} className="mb-2 px-3 py-2 clip-hud-sm font-mono text-[10px]" style={{ border: "1px solid var(--color-line)", background: "rgba(255,255,255,0.03)" }}>
          <button onClick={() => setOpen(open === r.id ? null : r.id)} className="w-full grid grid-cols-[120px_80px_80px_1fr_80px] gap-2 text-left cursor-pointer">
            <span className="text-[var(--color-dim)]">{new Date(r.started_at * 1000).toLocaleString()}</span>
            <span className="text-[var(--color-cyan)]">{r.trigger}</span>
            <StatusText status={r.status} />
            <span className="text-[var(--color-fg)] truncate">{r.prompt}</span>
            <span className="text-[var(--color-magenta)] text-right">{r.cost_tokens}</span>
          </button>
          {open === r.id && (
            <div className="mt-2 pt-2 border-t border-[var(--color-line)] whitespace-pre-wrap text-[var(--color-fg)]">
              {r.error ? `ERROR:\n${r.error}` : r.result || "sin resultado todavía"}
            </div>
          )}
        </div>
      ))}
      {runs.length === 0 && <Empty label="sin runs" />}
    </div>
  );
}

interface StreamPayload {
  kind?: string;
  text?: string;
  tool_name?: string;
  tool_args?: unknown;
  tool_result?: string;
  session_id?: string;
  seq?: number;
  final?: boolean;
}

interface RunDonePayload {
  status: string;
  result?: string;
  error?: string;
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

function RunNowTab({ agent, onDone }: { agent: MiniAgent; onDone: () => Promise<void> }) {
  const [prompt, setPrompt] = React.useState("");
  const [topic, setTopic] = React.useState("");
  const [ghost, setGhost] = React.useState<GhostBubbleData | null>(null);
  const [result, setResult] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const [running, setRunning] = React.useState(false);

  useTopic(topic, (_payload, env) => {
    if (env.type === "stream") {
      const ev = parsePayload<StreamPayload>(env.payload);
      if (!ev || ev.final) return;
      setGhost((g) => {
        const current = g ?? { id: `run-${topic}`, text: "", thinking: "", tools: [] };
        if (ev.kind === "text" && ev.text) return { ...current, text: current.text + ev.text };
        if (ev.kind === "thinking" && ev.text) return { ...current, thinking: current.thinking + ev.text };
        if (ev.kind === "tool_use") {
          const id = `${topic}-${ev.seq ?? Date.now()}`;
          const call: ToolCall = { id, name: ev.tool_name ?? "tool", args: ev.tool_args, status: "running" };
          return { ...current, tools: [...current.tools, call] };
        }
        if (ev.kind === "tool_result" && current.tools.length > 0) {
          const tools = current.tools.slice();
          tools[tools.length - 1] = { ...tools[tools.length - 1], status: "ok", resultPreview: (ev.tool_result ?? "").slice(0, 200) };
          return { ...current, tools };
        }
        return current;
      });
    }
    if (env.type === "run") {
      const done = parsePayload<RunDonePayload>(env.payload);
      setResult(done?.result || "");
      setError(done?.error || null);
      setRunning(false);
      setGhost(null);
      void onDone();
    }
  }, !!topic);

  async function run() {
    setResult("");
    setError(null);
    setGhost(null);
    setRunning(true);
    try {
      const res = await api.runAgentNow(agent.id, prompt);
      setTopic(res.topic);
    } catch (err) {
      setRunning(false);
      setError(err instanceof Error ? err.message : "error ejecutando agente");
    }
  }

  return (
    <div className="flex-1 min-h-0 grid grid-cols-[360px_1fr] gap-4">
      <div className="flex flex-col">
        <label className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">prompt opcional</label>
        <textarea value={prompt} onChange={(e) => setPrompt(e.target.value)} className="mt-1 min-h-[220px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} />
        <HudButton onClick={() => void run()} label={running ? "ejecutando…" : "ejecutar ahora"} accent="var(--color-orange)" className="mt-4 self-end" disabled={running} />
        {error && <ErrorBox msg={error} />}
      </div>
      <div className="overflow-y-auto pr-1">
        {ghost && <GhostBubble data={ghost} />}
        {running && !ghost && <Empty label="◂ mini-agente pensando…" />}
        {result && <div className="px-3 py-2 clip-hud-sm font-mono text-[12px] whitespace-pre-wrap text-[var(--color-fg)]" style={{ border: "1px solid var(--color-orange)", background: "rgba(255,159,67,0.06)" }}>{result}</div>}
        {!running && !ghost && !result && <Empty label="ejecutá un run manual para ver el stream" />}
      </div>
    </div>
  );
}

function AgentModal({ engines, onClose, onCreated }: { engines: EngineDef[]; onClose: () => void; onCreated: (a: MiniAgent) => void }) {
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [systemPrompt, setSystemPrompt] = React.useState("");
  const [engine, setEngine] = React.useState(engines[0]?.engine ?? "claude");
  const [error, setError] = React.useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    try {
      const a = await api.createAgent({ name, description, system_prompt: systemPrompt, engine });
      onCreated(a);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error creando agente");
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <form onSubmit={submit} className="w-[620px]">
        <HudPanel title="nuevo mini-agente" sub="persistente · manual/cron" accent="orange">
          <Field label="name" value={name} onChange={setName} />
          <Field label="description" value={description} onChange={setDescription} />
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">system_prompt</label>
          <textarea value={systemPrompt} onChange={(e) => setSystemPrompt(e.target.value)} className="mt-1 min-h-[180px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} />
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">engine</label>
          <select value={engine} onChange={(e) => setEngine(e.target.value)} className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }}>
            {engines.map((e) => <option key={e.engine} value={e.engine}>{e.engine}</option>)}
          </select>
          {error && <ErrorBox msg={error} />}
          <div className="mt-4 flex justify-end gap-2">
            <HudButton type="button" onClick={onClose} label="cancelar" accent="var(--color-dim)" />
            <HudButton type="submit" label="crear" accent="var(--color-orange)" />
          </div>
        </HudPanel>
      </form>
    </div>
  );
}

function Field({ label, value, onChange }: { label: string; value: string; onChange: (v: string) => void }) {
  return (
    <>
      <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">{label}</label>
      <input value={value} onChange={(e) => onChange(e.target.value)} className="mt-1 bg-transparent outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} />
    </>
  );
}

function HudButton({ label, accent, onClick, type = "button", className = "", disabled }: { label: string; accent: string; onClick?: () => void; type?: "button" | "submit"; className?: string; disabled?: boolean }) {
  return (
    <button type={type} onClick={onClick} disabled={disabled} className={`px-3 py-1 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed ${className}`} style={{ color: accent, border: `1px solid ${accent}`, background: `${accent}18` }}>
      {label}
    </button>
  );
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="clip-tag px-2 py-1" style={{ border: "1px solid var(--color-line)" }}>
      <div className="text-[var(--color-dim)] uppercase">{label}</div>
      <div className="text-[var(--color-fg)] truncate">{value}</div>
    </div>
  );
}

function StatusText({ status }: { status: string }) {
  const color = status === "ok" ? "var(--color-lime)" : status === "error" ? "var(--color-danger)" : "var(--color-orange)";
  return <span style={{ color }}>{status}</span>;
}

function Empty({ label }: { label: string }) {
  return <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">{label}</div>;
}

function ErrorBox({ msg }: { msg: string }) {
  return (
    <div className="mt-3 px-3 py-2 font-mono text-[10px] clip-hud-sm" style={{ background: "rgba(255, 92, 122, 0.08)", border: "1px solid rgba(255, 92, 122, 0.45)", color: "var(--color-danger)" }}>
      ✗ {msg}
    </div>
  );
}
