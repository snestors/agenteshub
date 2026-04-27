import * as React from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Check, ExternalLink, FileText, FolderKanban, Loader2, Pencil, RefreshCw, X } from "lucide-react";
import { api, DEFAULT_REASONING_EFFORTS, FALLBACK_ENGINES, type EngineDef, type OpenSpecChange, type OpenSpecChangeDetail, type OpenSpecSpec, type Project, type ProjectServiceStatus, type ProjectSession } from "@/lib/api";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { ProjectChat } from "@/components/ProjectChat";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

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
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [selected, setSelected] = React.useState<number>(routeSessionId || 0);
  const [sessionModalOpen, setSessionModalOpen] = React.useState(false);
  const [deleteTarget, setDeleteTarget] = React.useState<{ id: number; name: string } | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [tab, setTab] = React.useState<"chat" | "services" | "changes">("chat");

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
      <div className="flex-1 min-h-0 p-4">
        <HudPanel
          title={tab === "changes" ? "openspec changes" : tab === "services" ? "project services" : current ? `project chat · ${current.name}` : "project chat"}
          sub={tab === "changes" ? "openspec/changes · gates obligatorios" : tab === "services" ? ".agenthub/services.yaml" : current ? `topic project_session:${current.id}` : "sin sesión"}
          accent={tab === "changes" ? "lime" : tab === "services" ? "cyan" : "magenta"}
          className="min-h-0"
        >
          <div className="mb-3 flex gap-2">
            <TabButton active={tab === "chat"} onClick={() => setTab("chat")}>Chat</TabButton>
            <TabButton active={tab === "services"} onClick={() => setTab("services")}>Services</TabButton>
            <TabButton active={tab === "changes"} onClick={() => setTab("changes")}>Changes</TabButton>
          </div>
          {error && <ErrorBox msg={error} />}
          {tab === "changes" ? (
            <ProjectChanges projectId={projectId} visible={tab === "changes"} />
          ) : tab === "services" ? (
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


function ProjectChanges({ projectId, visible }: { projectId: number; visible: boolean }) {
  const [changes, setChanges] = React.useState<OpenSpecChange[]>([]);
  const [selected, setSelected] = React.useState<string>("");
  const [detail, setDetail] = React.useState<OpenSpecChangeDetail | null>(null);
  const [specs, setSpecs] = React.useState<OpenSpecSpec[]>([]);
  const [innerTab, setInnerTab] = React.useState<"proposal" | "design" | "tasks" | "verify" | "specs">("proposal");
  const [creating, setCreating] = React.useState(false);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const refreshList = React.useCallback(async () => {
    if (!visible) return;
    try {
      const list = await api.listOpenSpecChanges(projectId);
      setChanges(list);
      setSelected((cur) => cur || list[0]?.name || "");
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando changes");
    }
  }, [projectId, visible]);

  const refreshDetail = React.useCallback(async (name: string) => {
    if (!visible || !name) {
      setDetail(null);
      return;
    }
    try {
      const [d, s] = await Promise.all([
        api.getOpenSpecChange(projectId, name),
        api.listOpenSpecSpecs(projectId),
      ]);
      setDetail(d);
      setSpecs(s);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando detalle");
    }
  }, [projectId, visible]);

  React.useEffect(() => { void refreshList(); }, [refreshList]);
  React.useEffect(() => { void refreshDetail(selected); }, [refreshDetail, selected]);

  React.useEffect(() => {
    if (!visible || detail?.change.state !== "applying") return;
    const timer = window.setInterval(() => {
      void refreshList();
      void refreshDetail(detail.change.name);
    }, 1800);
    return () => window.clearInterval(timer);
  }, [detail?.change.name, detail?.change.state, refreshDetail, refreshList, visible]);

  async function created(change: OpenSpecChange) {
    setCreating(false);
    setSelected(change.name);
    await refreshList();
  }

  async function approve(dryRun = false) {
    if (!detail) return;
    try {
      setBusy(true);
      const next = await api.approveOpenSpecChange(projectId, detail.change.name, dryRun);
      setDetail(next);
      setSelected(next.change.name);
      setInnerTab(tabForState(next.change.state));
      await refreshList();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error aprobando");
    } finally {
      setBusy(false);
    }
  }

  async function reject() {
    if (!detail) return;
    try {
      setBusy(true);
      setDetail(await api.rejectOpenSpecChange(projectId, detail.change.name));
      await refreshList();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error rechazando");
    } finally {
      setBusy(false);
    }
  }

  async function feedback(text: string) {
    if (!detail) return;
    try {
      setBusy(true);
      const next = await api.feedbackOpenSpecChange(projectId, detail.change.name, text);
      setDetail(next);
      await refreshList();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error regenerando");
    } finally {
      setBusy(false);
    }
  }

  const active = detail?.change ?? changes.find((c) => c.name === selected) ?? null;

  return (
    <div className="min-h-0 flex-1 grid grid-cols-[280px_1fr] gap-3">
      <div className="min-h-0 flex flex-col">
        <div className="mb-3 flex items-center justify-between gap-2">
          <div className="font-mono text-[10px] text-[var(--color-dim)]">{changes.length} changes</div>
          <button onClick={() => setCreating(true)} className="px-2 py-1 clip-tag font-mono text-[9px] cursor-pointer" style={{ color: "var(--color-lime)", border: "1px solid var(--color-lime)", background: "rgba(163,255,78,0.08)" }}>
            + Nueva propuesta
          </button>
        </div>
        {error && <ErrorBox msg={error} />}
        <div className="flex-1 min-h-0 overflow-y-auto pr-1">
          {changes.map((c) => (
            <button key={c.id} onClick={() => { setSelected(c.name); setInnerTab(tabForState(c.state)); }} className="w-full text-left mb-2 px-3 py-2 clip-hud-sm font-mono cursor-pointer" style={{ border: `1px solid ${c.name === active?.name ? "var(--color-lime)" : "var(--color-line)"}`, background: c.name === active?.name ? "rgba(163,255,78,0.10)" : "rgba(255,255,255,0.03)" }}>
              <div className="flex items-center justify-between gap-2">
                <span className="text-[11px] text-[var(--color-fg)] truncate">{c.name}</span>
                <span className="text-[9px] text-[var(--color-dim)]">{rel(c.updated_at)}</span>
              </div>
              <div className="mt-1"><StateBadge state={c.state} /></div>
              <div className="mt-1 text-[9px] text-[var(--color-dim)] line-clamp-2">{c.description || "sin descripción"}</div>
            </button>
          ))}
          {changes.length === 0 && !error && <div className="h-[220px] flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">▸ sin changes · creá una propuesta</div>}
        </div>
      </div>

      <div className="min-h-0 flex flex-col overflow-hidden">
        {!detail ? (
          <div className="h-full flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">▸ seleccioná un change</div>
        ) : (
          <>
            <div className="mb-3 flex items-start justify-between gap-3">
              <div>
                <div className="font-display text-[18px] font-bold tracking-hud text-[var(--color-fg)]">{detail.change.name}</div>
                <div className="mt-1 font-mono text-[10px] text-[var(--color-dim)]">Descripción inicial: {detail.change.description}</div>
              </div>
              <StateBadge state={detail.change.state} />
            </div>
            {detail.change.state === "applying" && (
              <div className="mb-3 px-3 py-2 clip-hud-sm font-mono text-[10px] text-[var(--color-cyan)] flex items-center gap-2" style={{ border: "1px solid rgba(100,220,255,0.45)", background: "rgba(100,220,255,0.08)" }}>
                <Loader2 size={13} className="animate-spin" /> APPLYING · esperando sdd-verify…
              </div>
            )}
            <div className="mb-3 flex flex-wrap gap-2">
              <InnerTab active={innerTab === "proposal"} onClick={() => setInnerTab("proposal")}>Proposal {detail.proposal ? "✓" : ""}</InnerTab>
              <InnerTab active={innerTab === "design"} onClick={() => setInnerTab("design")}>Design {detail.design ? "✓" : ""}</InnerTab>
              <InnerTab active={innerTab === "tasks"} onClick={() => setInnerTab("tasks")}>Tasks {detail.tasks ? "✓" : ""}</InnerTab>
              <InnerTab active={innerTab === "verify"} onClick={() => setInnerTab("verify")}>Verify {detail.verify ? "✓" : ""}</InnerTab>
              <InnerTab active={innerTab === "specs"} onClick={() => setInnerTab("specs")}>Specs</InnerTab>
            </div>
            <div className="flex-1 min-h-0 overflow-y-auto pr-1">
              {innerTab === "specs" ? <SpecsView specs={specs} /> : <MarkdownDoc content={contentForTab(detail, innerTab)} empty={`sin ${innerTab}. Aprobá la fase anterior para generarlo.`} />}
            </div>
            <GateActions detail={detail} busy={busy} onApprove={approve} onReject={reject} onFeedback={feedback} />
          </>
        )}
      </div>
      {creating && <OpenSpecCreateModal projectId={projectId} onClose={() => setCreating(false)} onCreated={created} />}
    </div>
  );
}

function OpenSpecCreateModal({ projectId, onClose, onCreated }: { projectId: number; onClose: () => void; onCreated: (c: OpenSpecChange) => void }) {
  const [name, setName] = React.useState("");
  const [description, setDescription] = React.useState("");
  const [error, setError] = React.useState<string | null>(null);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    try {
      const c = await api.createOpenSpecChange(projectId, { name, description });
      onCreated(c);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error creando propuesta");
    }
  }
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <form onSubmit={submit} className="w-[620px]">
        <HudPanel title="nueva propuesta" sub="openspec change" accent="lime">
          <Field label="change_name" value={name} onChange={setName} placeholder="add-google-login" />
          <label className="mt-3 font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">descripción</label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)} className="mt-1 min-h-[110px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[12px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} placeholder="qué querés cambiar y por qué" />
          {error && <ErrorBox msg={error} />}
          <div className="mt-4 flex justify-end gap-2">
            <button type="button" onClick={onClose} className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-dim)] cursor-pointer" style={{ border: "1px solid var(--color-line)" }}>cancelar</button>
            <button type="submit" className="px-3 py-1 clip-tag font-mono text-[10px] text-[var(--color-lime)] cursor-pointer" style={{ border: "1px solid var(--color-lime)", background: "rgba(163,255,78,0.10)" }}>crear</button>
          </div>
        </HudPanel>
      </form>
    </div>
  );
}

function GateActions({ detail, busy, onApprove, onReject, onFeedback }: { detail: OpenSpecChangeDetail; busy: boolean; onApprove: (dryRun?: boolean) => void; onReject: () => void; onFeedback: (text: string) => void }) {
  const [editing, setEditing] = React.useState(false);
  const [text, setText] = React.useState("");
  const state = detail.change.state;
  if (state === "archived" || state === "rejected" || state === "applying") return null;
  const approveLabel = state === "pending_proposal" ? "Generar proposal" : state === "awaiting_approval_verify" ? "Aprobar y archivar" : state === "awaiting_approval_tasks" ? "Aprobar y aplicar" : "Aprobar y continuar";
  return (
    <div className="mt-3 pt-3 border-t border-[var(--color-line)]">
      {editing && (
        <div className="mb-3">
          <textarea value={text} onChange={(e) => setText(e.target.value)} className="w-full min-h-[74px] bg-transparent outline-none px-3 py-2 clip-hud-sm font-mono text-[11px] text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)" }} placeholder="qué ajuste pedís…" />
        </div>
      )}
      <div className="flex flex-wrap gap-2">
        <ActionButton onClick={() => onApprove(false)}><Check size={12} /> {busy ? "trabajando…" : approveLabel}</ActionButton>
        <ActionButton onClick={() => setEditing((v) => !v)}><Pencil size={12} /> Pedir ajuste</ActionButton>
        {editing && <ActionButton onClick={() => { onFeedback(text); setText(""); setEditing(false); }}><RefreshCw size={12} /> Re-generar</ActionButton>}
        <ActionButton onClick={onReject}><X size={12} /> Rechazar</ActionButton>
      </div>
    </div>
  );
}

function MarkdownDoc({ content, empty }: { content?: string; empty: string }) {
  if (!content?.trim()) return <div className="h-[180px] flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">▸ {empty}</div>;
  return (
    <div className="clip-hud-sm p-4 font-mono text-[12px] leading-6 text-[var(--color-fg)]" style={{ border: "1px solid var(--color-line)", background: "rgba(0,0,0,0.18)" }}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={{
        h1: ({ children }) => <h1 className="text-[18px] text-[var(--color-lime)] mb-3 font-bold">{children}</h1>,
        h2: ({ children }) => <h2 className="text-[14px] text-[var(--color-cyan)] mt-4 mb-2 font-bold">{children}</h2>,
        h3: ({ children }) => <h3 className="text-[12px] text-[var(--color-magenta)] mt-3 mb-1 font-bold">{children}</h3>,
        ul: ({ children }) => <ul className="list-disc pl-5 space-y-1">{children}</ul>,
        ol: ({ children }) => <ol className="list-decimal pl-5 space-y-1">{children}</ol>,
        code: ({ children }) => <code className="px-1 text-[var(--color-lime)] bg-white/5">{children}</code>,
      }}>{content}</ReactMarkdown>
    </div>
  );
}

function SpecsView({ specs }: { specs: OpenSpecSpec[] }) {
  if (specs.length === 0) return <div className="h-[180px] flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">▸ sin specs consolidadas todavía</div>;
  return <div className="space-y-3">{specs.map((s) => <div key={s.path}><div className="mb-1 font-mono text-[10px] text-[var(--color-cyan)] flex items-center gap-1"><FileText size={11} /> {s.capability}</div><MarkdownDoc content={s.content} empty="spec vacío" /></div>)}</div>;
}

function InnerTab({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return <button onClick={onClick} className="px-2 py-1 clip-tag font-mono text-[9px] tracking-hud uppercase cursor-pointer" style={{ color: active ? "var(--color-lime)" : "var(--color-dim)", border: `1px solid ${active ? "var(--color-lime)" : "var(--color-line)"}`, background: active ? "rgba(163,255,78,0.10)" : "rgba(255,255,255,0.03)" }}>{children}</button>;
}

function StateBadge({ state }: { state: string }) {
  const color = state === "archived" ? "var(--color-lime)" : state === "rejected" ? "var(--color-danger)" : state === "applying" ? "var(--color-cyan)" : "var(--color-magenta)";
  return <span className="font-mono text-[9px] uppercase tracking-hud" style={{ color }}>● {state}</span>;
}

function contentForTab(detail: OpenSpecChangeDetail, tab: "proposal" | "design" | "tasks" | "verify" | "specs"): string {
  if (tab === "proposal") return detail.proposal ?? "";
  if (tab === "design") return detail.design ?? "";
  if (tab === "tasks") return detail.tasks ?? "";
  if (tab === "verify") return detail.verify ?? "";
  return "";
}

function tabForState(state: string): "proposal" | "design" | "tasks" | "verify" | "specs" {
  if (state.includes("design")) return "design";
  if (state.includes("tasks")) return "tasks";
  if (state.includes("verify") || state === "applying") return "verify";
  if (state === "archived") return "verify";
  return "proposal";
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70" onClick={onClose}>
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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70">
      <form onSubmit={submit} className="w-[520px]">
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
