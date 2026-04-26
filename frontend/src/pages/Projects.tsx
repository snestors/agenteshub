import * as React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ExternalLink, FolderKanban, Plus, RefreshCw, TerminalSquare } from "lucide-react";
import { api, FALLBACK_ENGINES, type EngineDef, type Project, type ProjectServiceStatus, type ProjectSession } from "@/lib/api";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { ProjectChat } from "@/components/ProjectChat";

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
        breadcrumb={[{ label: "AgentHub" }, { label: "Projects" }]}
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

      <div className="flex-1 min-h-0 p-4 overflow-y-auto">
        {error && <ErrorBox msg={error} />}
        <div className="grid grid-cols-3 gap-4">
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
  const [selected, setSelected] = React.useState<number>(routeSessionId || 0);
  const [newName, setNewName] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  const [tab, setTab] = React.useState<"chat" | "services">("chat");

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

  const current = sessions.find((s) => s.id === selected) ?? null;

  async function createSession() {
    const name = newName.trim() || `session-${new Date().toISOString().slice(0, 16).replace(/[-:T]/g, "")}`;
    try {
      const s = await api.createProjectSession(projectId, { name });
      setNewName("");
      await refresh();
      nav(`/projects/${projectId}/sessions/${s.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error creando sesión");
    }
  }

  function selectSession(id: number) {
    setSelected(id);
    nav(`/projects/${projectId}/sessions/${id}`);
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Projects" }, { label: project?.name ?? String(projectId) }]}
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
      <div className="flex-1 min-h-0 p-4 grid grid-cols-[310px_1fr] gap-4">
        <HudPanel
          title="sessions"
          sub={`${sessions.length} · ${project?.default_engine ?? "engine"}`}
          accent="lime"
        >
          <div className="font-mono text-[10px] text-[var(--color-dim)] mb-3 break-all">
            {project?.path}
          </div>
          <div className="flex gap-2 mb-3">
            <input
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="nueva sesión"
              className="flex-1 bg-transparent outline-none px-2 py-1 clip-tag font-mono text-[10px] text-[var(--color-fg)]"
              style={{ border: "1px solid var(--color-line)" }}
            />
            <button
              onClick={() => void createSession()}
              className="px-2 clip-tag cursor-pointer"
              style={{ color: "var(--color-lime)", border: "1px solid var(--color-lime)" }}
              title="nueva sesión"
            >
              <Plus size={13} />
            </button>
          </div>
          <div className="flex-1 min-h-0 overflow-y-auto pr-1">
            {sessions.map((s) => {
              const active = s.id === selected;
              return (
                <button
                  key={s.id}
                  onClick={() => selectSession(s.id)}
                  className="w-full text-left mb-2 px-3 py-2 clip-hud-sm font-mono cursor-pointer"
                  style={{
                    background: active ? "rgba(163,255,78,0.10)" : "rgba(255,255,255,0.03)",
                    border: `1px solid ${active ? "var(--color-lime)" : "var(--color-line)"}`,
                  }}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="text-[var(--color-fg)] text-[11px] truncate">{s.name}</span>
                    <span className="text-[9px] text-[var(--color-dim)]">{rel(s.last_active_at || s.created_at)}</span>
                  </div>
                  <div className="mt-1 text-[9px] text-[var(--color-cyan)] flex items-center gap-1">
                    <TerminalSquare size={10} /> {s.engine}
                  </div>
                  <div className="mt-1 text-[9px] text-[var(--color-dim)] line-clamp-2">
                    {s.summary || (s.session_id ? s.session_id : "sin CLI session todavía")}
                  </div>
                </button>
              );
            })}
          </div>
        </HudPanel>

        <HudPanel
          title={tab === "services" ? "project services" : current ? `project chat · ${current.name}` : "project chat"}
          sub={tab === "services" ? ".agenthub/services.yaml" : current ? `topic project_session:${current.id}` : "sin sesión"}
          accent={tab === "services" ? "cyan" : "magenta"}
          className="min-h-0"
        >
          <div className="mb-3 flex gap-2">
            <TabButton active={tab === "chat"} onClick={() => setTab("chat")}>Chat</TabButton>
            <TabButton active={tab === "services"} onClick={() => setTab("services")}>Services</TabButton>
          </div>
          {error && <ErrorBox msg={error} />}
          {tab === "services" ? (
            <ProjectServices projectId={projectId} visible={tab === "services"} />
          ) : current ? (
            <ProjectChat projectId={projectId} sessionId={current.id} sessionName={current.name} />
          ) : (
            <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
              ▸ creá una sesión para empezar
            </div>
          )}
        </HudPanel>
      </div>
    </div>
  );
}

function TabButton({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      onClick={onClick}
      className="px-3 py-1 clip-tag font-mono text-[10px] tracking-hud uppercase cursor-pointer"
      style={{
        color: active ? "var(--color-cyan)" : "var(--color-dim)",
        border: `1px solid ${active ? "var(--color-cyan)" : "var(--color-line)"}`,
        background: active ? "rgba(100,220,255,0.10)" : "rgba(255,255,255,0.03)",
      }}
    >
      {children}
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <form onSubmit={submit} className="w-[560px]">
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
