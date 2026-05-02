import * as React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ExternalLink, FolderKanban, MessageSquare, RefreshCw, Server, Settings2, X } from "lucide-react";
import {
  api,
  DEFAULT_REASONING_EFFORTS,
  FALLBACK_ENGINES,
  type AgentStatus,
  type ConversationRuntime,
  type EngineDef,
  type Project,
  type ProjectFeatures,
  type ProjectServiceStatus,
  type ProjectSession,
  type RealtimeResponse,
} from "@/lib/api";
import { ChatHUD } from "@/components/ChatHUD";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { ProjectChat } from "@/components/ProjectChat";
import { cancelProjectSession, toggleProjectSessionConfig } from "@/lib/projectSessionConfig";

function rel(ts?: number): string {
  if (!ts) return "—";
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts);
  if (s < 60) return "ahora";
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export function Projects() {
  const params = useParams();
  const id = params.id ? Number(params.id) : 0;
  const sid = params.sid ? Number(params.sid) : 0;
  return id > 0 ? <ProjectDetail projectId={id} routeSessionId={sid} /> : <ProjectList />;
}

function ProjectList() {
  const nav = useNavigate();
  const [projects, setProjects] = React.useState<Project[]>([]);
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [open, setOpen] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    try {
      setProjects(await api.listProjects());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando proyectos");
    }
  }, []);

  React.useEffect(() => {
    void refresh();
    void api.listEngines().then(setEngines).catch(() => undefined);
  }, [refresh]);

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub", href: "/" }, { label: "Projects" }]}
        status={error ? { label: "ERROR", tone: "danger" } : { label: "READY", tone: "ok" }}
        right={
          <button
            onClick={() => setOpen(true)}
            className="px-3 py-1 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer"
            style={{
              color: "var(--color-lime)",
              border: "1px solid var(--color-lime)",
              background: "rgba(163,255,78,0.10)",
            }}
          >
            + nuevo proyecto
          </button>
        }
      />

      <div className="flex-1 min-h-0 p-2 overflow-y-auto sm:p-4">
        {error && <ErrorBox msg={error} />}
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3 xl:gap-4">
          {projects.map((p) => (
            <button
              key={p.id}
              onClick={() => nav(`/projects/${p.id}`)}
              className="text-left cursor-pointer"
            >
              <HudPanel accent="lime" className="min-h-[190px] hover:opacity-90 transition-opacity">
                <div className="flex items-start justify-between gap-3">
                  <FolderKanban size={22} style={{ color: "var(--color-lime)" }} />
                  <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight">
                    upd {rel(p.updated_at)}
                  </span>
                </div>
                <div className="mt-3 font-display text-[18px] font-bold tracking-hud text-[var(--color-fg)]">
                  {p.name}
                </div>
                <div className="mt-1 font-mono text-[10px] text-[var(--color-cyan)] break-all">
                  {p.path}
                </div>
                <div className="mt-3 font-mono text-[11px] text-[var(--color-dim)] line-clamp-2">
                  {p.description || "sin descripción"}
                </div>
                <div className="mt-auto pt-3 flex items-center justify-between font-mono text-[10px]">
                  <span className="text-[var(--color-magenta)]">{p.sessions_count ?? 0} sesiones</span>
                  <span className="text-[var(--color-lime)]">{p.default_engine}</span>
                </div>
              </HudPanel>
            </button>
          ))}
        </div>

        {projects.length === 0 && !error && (
          <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
            ▸ sin proyectos · creá el primero
          </div>
        )}
      </div>

      {open && (
        <ProjectModal
          engines={engines}
          onClose={() => setOpen(false)}
          onCreated={(p) => {
            setOpen(false);
            nav(`/projects/${p.id}`);
          }}
        />
      )}
    </div>
  );
}

function ProjectDetail({ projectId, routeSessionId }: { projectId: number; routeSessionId: number }) {
  const nav = useNavigate();
  const [project, setProject] = React.useState<Project | null>(null);
  const [sessions, setSessions] = React.useState<ProjectSession[]>([]);
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [selected, setSelected] = React.useState<number>(routeSessionId || 0);
  const [sessionModalOpen, setSessionModalOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<{ id: number; name: string } | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [tab, setTab] = React.useState<"chat" | "services">("chat");
  const [projectRunActive, setProjectRunActive] = React.useState(false);

  const refresh = React.useCallback(async () => {
    try {
      const res = await api.getProject(projectId);
      setProject(res.project);
      setSessions(res.sessions);
      const wanted = routeSessionId || selected || res.sessions[0]?.id || 0;
      if (wanted && res.sessions.some((s) => s.id === wanted)) {
        setSelected(wanted);
      } else if (res.sessions[0]) {
        setSelected(res.sessions[0].id);
      }
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando proyecto");
    }
  }, [projectId, routeSessionId, selected]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  React.useEffect(() => {
    void api.listEngines().then((list) => {
      if (list.length > 0) setEngines(list);
    }).catch(() => undefined);
  }, []);

  const current = sessions.find((s) => s.id === selected) ?? null;

  React.useEffect(() => {
    setProjectRunActive(false);
  }, [selected]);

  function openCurrentSessionConfig() {
    if (!current) return;
    setTab("chat");
    window.requestAnimationFrame(() => toggleProjectSessionConfig(current.id));
  }

  function cancelCurrentProjectRun() {
    if (!current) return;
    cancelProjectSession(current.id);
  }

  async function createSession(payload: { engine: string; model: string; reasoning_effort: string }) {
    try {
      const s = await api.createProjectSession(projectId, { name: "", ...payload });
      await refresh();
      nav(`/projects/${projectId}/sessions/${s.id}`);
      setSessionModalOpen(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error creando sesión");
    }
  }

  function selectSession(id: number) {
    setSelected(id);
    nav(`/projects/${projectId}/sessions/${id}`);
  }

  async function confirmDeleteSession() {
    if (!deleteTarget) return;
    const { id } = deleteTarget;
    const remaining = sessions.filter((s) => s.id !== id);
    setDeleteTarget(null);
    try {
      await api.deleteProjectSession(projectId, id);
      await refresh();
      if (selected === id) {
        if (remaining[0]) {
          selectSession(remaining[0].id);
        } else {
          setSelected(0);
          nav(`/projects/${projectId}`);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "error eliminando sesión");
    }
  }

  // ─── HUD lateral (Sprint A2 + 0.5.2 reactivity fix) ───────────────
  // Polling cada 4s; bajamos de 6s para que feature_list y los counters de
  // RUNTIME sean visibly reactivos. /api/runs nutre los counters AGENTS ·
  // RUNTIME que antes quedaban en cero.
  const [hudStatus, setHudStatus] = React.useState<AgentStatus | null>(null);
  const [hudRuntime, setHudRuntime] = React.useState<ConversationRuntime | null>(null);
  const [hudRealtime, setHudRealtime] = React.useState<RealtimeResponse | null>(null);
  const [hudFeatures, setHudFeatures] = React.useState<ProjectFeatures | null>(null);
  const [hudRunsMain, setHudRunsMain] = React.useState(0);
  const [hudRunsProject, setHudRunsProject] = React.useState(0);
  React.useEffect(() => {
    if (tab !== "chat" || !current) return;
    let cancelled = false;
    const tick = async () => {
      try {
        const [status, runtime, realtime, features, runs] = await Promise.allSettled([
          api.agentStatus(),
          api.getProjectRuntime(projectId, current.id),
          api.getUsageRealtime(),
          api.getProjectFeatures(projectId),
          api.getRunsStatus(),
        ]);
        if (cancelled) return;
        if (status.status === "fulfilled") setHudStatus(status.value);
        if (runtime.status === "fulfilled") setHudRuntime(runtime.value);
        if (realtime.status === "fulfilled") setHudRealtime(realtime.value);
        if (features.status === "fulfilled") setHudFeatures(features.value);
        if (runs.status === "fulfilled") {
          setHudRunsMain(runs.value.runs?.main ?? 0);
          setHudRunsProject(runs.value.runs?.project ?? 0);
        }
      } catch {
        /* ignore — HUD is best-effort */
      }
    };
    tick();
    const handle = window.setInterval(tick, 4000);
    return () => {
      cancelled = true;
      window.clearInterval(handle);
    };
  }, [tab, current, projectId]);

  async function scaffoldProjectHarness() {
    try {
      await fetch(`/api/projects/${projectId}/harness/scaffold`, {
        method: "POST",
        credentials: "include",
      });
      // Re-fetch features so the HUD flips from "sin harness" to the populated state.
      const features = await api.getProjectFeatures(projectId);
      setHudFeatures(features);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error scaffolding harness");
    }
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub", href: "/" }, { label: "Projects", href: "/projects" }, { label: project?.name ?? String(projectId) }]}
        status={error ? { label: "ERROR", tone: "danger" } : { label: current ? "LIVE" : "IDLE", tone: current ? "ok" : "warn" }}
        right={
          <button
            onClick={() => nav("/projects")}
            className="font-mono text-[10px] text-[var(--color-dim)] hover:text-[var(--color-cyan)] cursor-pointer"
          >
            ← lista
          </button>
        }
      />
      <div className="flex-1 min-h-0 flex flex-row overflow-hidden">
       <div className="flex-1 min-h-0 p-2 sm:p-4">
        <HudPanel
          title={tab === "services" ? "project services" : current ? `project chat · ${current.name}` : "project chat"}
          sub={tab === "services" ? ".agenthub/services.yaml" : current ? `topic project_session:${current.id}` : "sin sesión"}
          accent={tab === "services" ? "cyan" : "magenta"}
          className="min-h-0"
        >
          <div className="mb-2 flex items-center gap-1.5">
            <TabButton active={tab === "chat"} onClick={() => setTab("chat")} icon={MessageSquare} label="Chat" />
            <TabButton active={tab === "services"} onClick={() => setTab("services")} icon={Server} label="Services" />
            {current && tab === "chat" && projectRunActive && (
              <button
                type="button"
                onClick={cancelCurrentProjectRun}
                className="inline-flex h-8 min-w-8 items-center justify-center clip-tag cursor-pointer transition-opacity hover:opacity-85"
                style={{
                  color: "var(--color-danger)",
                  border: "1px solid rgba(255,92,122,0.55)",
                  background: "rgba(255,92,122,0.10)",
                }}
                title="Cancelar ejecución"
                aria-label="Cancelar ejecución"
              >
                <X size={13} strokeWidth={1.8} />
              </button>
            )}
            {current && (
              <button
                type="button"
                onClick={openCurrentSessionConfig}
                className="inline-flex h-8 min-w-8 items-center justify-center clip-tag cursor-pointer transition-opacity hover:opacity-85"
                style={{
                  color: "var(--color-magenta)",
                  border: "1px solid rgba(255,78,214,0.48)",
                  background: "rgba(255,78,214,0.08)",
                }}
                title="Configurar sesión y modelo"
                aria-label="Configurar sesión y modelo"
              >
                <Settings2 size={13} strokeWidth={1.8} />
              </button>
            )}
          </div>
          {error && <ErrorBox msg={error} />}
          {tab === "services" ? (
            <ProjectServices projectId={projectId} visible={tab === "services"} />
          ) : current ? (
            <ProjectChat
              projectId={projectId}
              sessionId={current.id}
              sessionName={current.name}
              engine={current.engine}
              model={current.model}
              reasoningEffort={current.reasoning_effort}
              sessions={sessions}
              onSessionSelect={selectSession}
              onCreateSession={() => setSessionModalOpen(true)}
              onDeleteSession={(id) => {
                const s = sessions.find((row) => row.id === id);
                if (s) setDeleteTarget({ id: s.id, name: s.name });
              }}
              onSessionConfigChange={(patch) =>
                setSessions((prev) =>
                  prev.map((s) => (s.id === current.id ? { ...s, ...patch } : s))
                )
              }
              onRunningChange={setProjectRunActive}
            />
          ) : (
            <div className="h-full flex flex-col items-center justify-center gap-3 font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
              <div>▸ creá una sesión para empezar</div>
              <button
                type="button"
                onClick={() => setSessionModalOpen(true)}
                className="px-3 py-1 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer"
                style={{ color: "var(--color-lime)", border: "1px solid var(--color-lime)", background: "rgba(163,255,78,0.10)" }}
              >
                + nueva sesión
              </button>
            </div>
          )}
        </HudPanel>
       </div>
       {tab === "chat" && current && (
         <ChatHUD
           scope="project"
           scopeKey={`project:${projectId}:${current.id}`}
           status={hudStatus}
           runtime={hudRuntime}
           realtimeUsage={hudRealtime}
           projectFeatures={hudFeatures}
           runningMain={hudRunsMain}
           runningProject={hudRunsProject}
           wsConnected={true /* TODO Sprint B: real ws status */}
           onScaffoldHarness={scaffoldProjectHarness}
         />
       )}
      </div>
      {sessionModalOpen && (
        <ProjectSessionModal
          engines={engines}
          projectDefaultEngine={project?.default_engine}
          onClose={() => setSessionModalOpen(false)}
          onCreate={(payload) => void createSession(payload)}
        />
      )}
      {deleteTarget && (
        <DeleteSessionModal
          name={deleteTarget.name}
          onConfirm={() => void confirmDeleteSession()}
          onClose={() => setDeleteTarget(null)}
        />
      )}
    </div>
  );
}


function TabButton({
  active,
  onClick,
  icon: Icon,
  label,
}: {
  active: boolean;
  onClick: () => void;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number }>;
  label: string;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="inline-flex h-8 min-w-8 items-center justify-center gap-1.5 px-2 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer sm:h-auto sm:px-3 sm:py-1"
      style={{
        color: active ? "var(--color-cyan)" : "var(--color-dim)",
        border: `1px solid ${active ? "var(--color-cyan)" : "var(--color-line)"}`,
        background: active ? "rgba(100,220,255,0.10)" : "rgba(255,255,255,0.03)",
      }}
      title={label}
      aria-label={label}
    >
      <Icon size={13} strokeWidth={1.7} />
      <span className="hidden sm:inline">{label}</span>
    </button>
  );
}


function ProjectServices({ projectId, visible }: { projectId: number; visible: boolean }) {
  const [services, setServices] = React.useState<ProjectServiceStatus[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const refresh = React.useCallback(async () => {
    if (!visible) return;
    try {
      setLoading(true);
      setServices(await api.listProjectServices(projectId));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando servicios");
    } finally {
      setLoading(false);
    }
  }, [projectId, visible]);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  React.useEffect(() => {
    if (!visible) return;
    const timer = window.setInterval(() => void refresh(), 5000);
    return () => window.clearInterval(timer);
  }, [refresh, visible]);

  async function reload() {
    try {
      setLoading(true);
      setServices(await api.reloadProjectServices(projectId));
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error recargando servicios");
    } finally {
      setLoading(false);
    }
  }

  async function action(idx: number, actionName: "start" | "stop" | "restart") {
    try {
      await api.projectServiceAction(projectId, idx, actionName);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error ejecutando acción");
    }
  }

  return (
    <div className="min-h-0 flex-1 overflow-y-auto pr-1">
      <div className="mb-3 flex items-center justify-between">
        <div className="font-mono text-[10px] text-[var(--color-dim)]">
          {loading ? "actualizando…" : `${services.length} servicios declarados`}
        </div>
        <button
          onClick={() => void reload()}
          className="px-2 py-1 clip-tag font-mono text-[10px] cursor-pointer flex items-center gap-1"
          style={{ color: "var(--color-cyan)", border: "1px solid var(--color-cyan)" }}
        >
          <RefreshCw size={11} /> reload
        </button>
      </div>
      {error && <ErrorBox msg={error} />}
      {services.length === 0 && !error && (
        <div className="h-[220px] flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
          ▸ sin servicios · completá .agenthub/services.yaml
        </div>
      )}
      <div className="space-y-3">
        {services.map((svc, idx) => (
          <ServiceCard key={`${svc.kind}-${idx}-${svc.unit || svc.container || svc.hostname || svc.command}`} svc={svc} idx={idx} onAction={action} />
        ))}
      </div>
    </div>
  );
}

function ServiceCard({
  svc,
  idx,
  onAction,
}: {
  svc: ProjectServiceStatus;
  idx: number;
  onAction: (idx: number, actionName: "start" | "stop" | "restart") => Promise<void>;
}) {
  const border = svc.status === "active"
    ? "rgba(163,255,78,0.65)"
    : svc.status === "failed"
      ? "rgba(255,92,122,0.75)"
      : "var(--color-line)";
  const identity = svc.unit || svc.container || svc.hostname || svc.command || "sin id";
  const canAct = svc.kind === "systemd" || svc.kind === "docker";
  const healthLabel = svc.health_url || svc.health_cmd || "sin healthcheck";

  return (
    <div
      className="clip-hud-sm px-4 py-3 font-mono"
      style={{ border: `1px solid ${border}`, background: "rgba(255,255,255,0.035)" }}
    >
      <div className="flex items-start justify-between gap-3">
        <div>
          <div className="text-[13px] font-bold text-[var(--color-fg)]">{svc.description || identity}</div>
          <div className="mt-1 text-[9px] text-[var(--color-dim)] tracking-hud-tight">
            [{svc.kind} · {identity}]
          </div>
        </div>
        <StatusDot status={svc.status} />
      </div>

      <div className="mt-3 text-[10px] text-[var(--color-dim)]">
        ● <span className="text-[var(--color-fg)]">{svc.status}</span>
        {" · since "}{rel(svc.since)}
        {" · CPU "}{(svc.cpu_pct ?? 0).toFixed(1)}%
        {" · Mem "}{(svc.mem_mb ?? 0).toFixed(1)}MB
      </div>
      <div className="mt-2 grid grid-cols-[90px_1fr] gap-2 text-[10px]">
        <div className="text-[var(--color-dim)] tracking-hud">HEALTH</div>
        <div className={svc.health_ok ? "text-[var(--color-lime)]" : "text-[var(--color-danger)]"}>
          {svc.health_ok ? "✓" : "✗"} {healthLabel}{svc.health_error ? ` · ${svc.health_error}` : ""}
        </div>
        <div className="text-[var(--color-dim)] tracking-hud">PUBLIC URL</div>
        <div>
          {svc.public_url ? (
            <a className="text-[var(--color-cyan)] inline-flex items-center gap-1" href={svc.public_url} target="_blank" rel="noreferrer">
              🌐 {svc.public_url} <ExternalLink size={10} />
            </a>
          ) : (
            <span className="text-[var(--color-dim)]">—</span>
          )}
        </div>
      </div>

      <div className="mt-3 flex gap-2">
        {canAct ? (
          <>
            <ActionButton onClick={() => void onAction(idx, "start")}>start</ActionButton>
            <ActionButton onClick={() => void onAction(idx, "stop")}>stop</ActionButton>
            <ActionButton onClick={() => void onAction(idx, "restart")}>restart</ActionButton>
          </>
        ) : (
          <span className="text-[9px] text-[var(--color-dim)]">acciones read-only en v1</span>
        )}
      </div>
    </div>
  );
}

function StatusDot({ status }: { status: string }) {
  const color = status === "active" ? "var(--color-lime)" : status === "failed" ? "var(--color-danger)" : "var(--color-dim)";
  return (
    <span className="text-[10px] uppercase tracking-hud" style={{ color }}>
      ● {status}
    </span>
  );
}

function ActionButton({ onClick, children }: { onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className="px-2 py-1 clip-tag font-mono text-[9px] text-[var(--color-lime)] cursor-pointer"
      style={{ border: "1px solid rgba(163,255,78,0.55)", background: "rgba(163,255,78,0.08)" }}
    >
      {children}
    </button>
  );
}

function DeleteSessionModal({
  name,
  onConfirm,
  onClose,
}: {
  name: string;
  onConfirm: () => void;
  onClose: () => void;
}) {
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70 py-4" onClick={onClose}>
      <div
        className="clip-hud font-mono"
        style={{
          background: "rgba(10,15,36,0.97)",
          border: "1px solid rgba(255,92,122,0.55)",
          boxShadow: "0 0 24px rgba(255,92,122,0.2)",
          minWidth: 340,
          padding: "20px 24px",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <p className="text-[10px] uppercase tracking-hud mb-1" style={{ color: "var(--color-danger)" }}>
          confirmar eliminación
        </p>
        <p className="text-[13px] text-[var(--color-fg)] mb-1">
          {name}
        </p>
        <p className="text-[10px] text-[var(--color-dim)] mb-5">
          Se elimina la sesión y su historial de mensajes. Esta acción no se puede deshacer.
        </p>
        <div className="flex gap-3 justify-end">
          <button
            onClick={onClose}
            className="px-4 py-1.5 clip-tag font-mono text-[11px] tracking-hud cursor-pointer"
            style={{ color: "var(--color-dim)", border: "1px solid var(--color-line)" }}
          >
            cancelar
          </button>
          <button
            onClick={onConfirm}
            className="px-4 py-1.5 clip-tag font-mono text-[11px] tracking-hud cursor-pointer"
            style={{
              color: "var(--color-danger)",
              border: "1px solid rgba(255,92,122,0.6)",
              background: "rgba(255,92,122,0.10)",
            }}
          >
            eliminar
          </button>
        </div>
      </div>
    </div>
  );
}

function ProjectSessionModal({
  engines,
  projectDefaultEngine,
  onClose,
  onCreate,
}: {
  engines: EngineDef[];
  projectDefaultEngine?: string;
  onClose: () => void;
  onCreate: (payload: { engine: string; model: string; reasoning_effort: string }) => void;
}) {
  const initialEngine = projectDefaultEngine || engines[0]?.engine || FALLBACK_ENGINES[0]?.engine || "claude";
  const [engine, setEngine] = React.useState(initialEngine);
  const engineDef = engines.find((e) => e.engine === engine) ?? engines[0] ?? FALLBACK_ENGINES[0];
  const modelOptions = engineDef?.models ?? [];
  const effortOptions = engineDef?.reasoning_efforts?.length ? engineDef.reasoning_efforts : DEFAULT_REASONING_EFFORTS;
  const [model, setModel] = React.useState(modelOptions[0] ?? "");
  const [effort, setEffort] = React.useState(effortOptions.includes("medium") ? "medium" : effortOptions[0] ?? "");

  React.useEffect(() => {
    if (!modelOptions.includes(model)) {
      setModel(modelOptions[0] ?? "");
    }
    if (!effortOptions.includes(effort)) {
      setEffort(effortOptions.includes("medium") ? "medium" : effortOptions[0] ?? "");
    }
  }, [effort, effortOptions, model, modelOptions]);

  function submit(e: React.FormEvent) {
    e.preventDefault();
    onCreate({
      engine,
      model: model || modelOptions[0] || "",
      reasoning_effort: effort || "medium",
    });
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70 py-4">
      <form onSubmit={submit} className="mx-4 w-full max-w-[520px]">
        <HudPanel title="nueva sesión" sub="nombre automático · contexto aislado por engine" accent="lime">
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
            engine
          </label>
          <select
            value={engine}
            onChange={(e) => {
              const next = e.target.value;
              setEngine(next);
              const def = engines.find((item) => item.engine === next);
              setModel(def?.models[0] ?? "");
              const efforts = def?.reasoning_efforts?.length ? def.reasoning_efforts : DEFAULT_REASONING_EFFORTS;
              setEffort(efforts.includes("medium") ? "medium" : efforts[0] ?? "");
            }}
            className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-cyan)]"
            style={{ border: "1px solid rgba(94,240,255,0.45)" }}
          >
            {engines.map((e) => (
              <option key={e.engine} value={e.engine}>{e.engine}</option>
            ))}
          </select>

          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
            modelo
          </label>
          <select
            value={model}
            onChange={(e) => setModel(e.target.value)}
            className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-lime)]"
            style={{ border: "1px solid rgba(163,255,78,0.45)" }}
          >
            {modelOptions.map((m) => (
              <option key={m} value={m}>{m}</option>
            ))}
          </select>

          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
            reasoning effort
          </label>
          <select
            value={effort}
            onChange={(e) => setEffort(e.target.value)}
            className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-orange)]"
            style={{ border: "1px solid rgba(255,159,67,0.45)" }}
          >
            {effortOptions.map((item) => (
              <option key={item} value={item}>{item}</option>
            ))}
          </select>

          <div className="mt-4 flex justify-end gap-2">
            <button type="button" onClick={onClose} className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-dim)] cursor-pointer" style={{ border: "1px solid var(--color-line)" }}>
              cancelar
            </button>
            <button type="submit" className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-lime)] cursor-pointer" style={{ border: "1px solid var(--color-lime)", background: "rgba(163,255,78,0.10)" }}>
              crear sesión
            </button>
          </div>
        </HudPanel>
      </form>
    </div>
  );
}

function ProjectModal({
  engines,
  onClose,
  onCreated,
}: {
  engines: EngineDef[];
  onClose: () => void;
  onCreated: (p: Project) => void;
}) {
  const [name, setName] = React.useState("");
  const [path, setPath] = React.useState("/home/nestor/agenthub");
  const [description, setDescription] = React.useState("");
  const [engine, setEngine] = React.useState(engines[0]?.engine ?? "claude");
  const [error, setError] = React.useState<string | null>(null);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    try {
      const p = await api.createProject({ name, path, description, default_engine: engine });
      onCreated(p);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error creando proyecto");
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/70 py-4">
      <form onSubmit={submit} className="mx-4 w-full max-w-[560px]">
        <HudPanel title="nuevo proyecto" sub="path validado en backend" accent="lime">
          <Field label="name" value={name} onChange={setName} placeholder="agenthub" />
          <Field label="path" value={path} onChange={setPath} placeholder="/home/nestor/agenthub" />
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
            description
          </label>
          <textarea
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            className="mt-1 min-h-[80px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[12px] text-[var(--color-fg)]"
            style={{ border: "1px solid var(--color-line)" }}
          />
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
            default_engine
          </label>
          <select
            value={engine}
            onChange={(e) => setEngine(e.target.value)}
            className="mt-1 bg-[var(--color-bg-2)] outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-fg)]"
            style={{ border: "1px solid var(--color-line)" }}
          >
            {engines.map((e) => (
              <option key={e.engine} value={e.engine}>{e.engine}</option>
            ))}
          </select>
          {error && <ErrorBox msg={error} />}
          <div className="mt-4 flex justify-end gap-2">
            <button type="button" onClick={onClose} className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-dim)] cursor-pointer" style={{ border: "1px solid var(--color-line)" }}>
              cancelar
            </button>
            <button type="submit" className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-lime)] cursor-pointer" style={{ border: "1px solid var(--color-lime)", background: "rgba(163,255,78,0.10)" }}>
              crear
            </button>
          </div>
        </HudPanel>
      </form>
    </div>
  );
}

function Field({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
}) {
  return (
    <>
      <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
        {label}
      </label>
      <input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="mt-1 bg-transparent outline-none px-3 py-2 clip-tag font-mono text-[12px] text-[var(--color-fg)]"
        style={{ border: "1px solid var(--color-line)" }}
      />
    </>
  );
}

function ErrorBox({ msg }: { msg: string }) {
  return (
    <div
      className="mb-3 px-3 py-2 font-mono text-[10px] clip-hud-sm"
      style={{
        background: "rgba(255, 92, 122, 0.08)",
        border: "1px solid rgba(255, 92, 122, 0.45)",
        color: "var(--color-danger)",
      }}
    >
      ✗ {msg}
    </div>
  );
}
