// API client for AgentHub Go backend.
// All requests use cookie auth (httpOnly, set on /api/auth/login).
// Send credentials:"include" so the cookie travels.

export interface LoginResponse {
  need_totp: boolean;
  token?: string;
}

export interface Me {
  id: number;
  username: string;
  last_login?: number;
}

export interface SendMessageResponse {
  id: number;
  reply: string;
  session_id: string;
  tokens: number;
}

export interface AgentStatus {
  engine: string;
  model: string;
  ctx_window: number;
  ctx_used: number;
  ctx_pct: number;
  cost_usd?: number;
  session_id?: string;
  wa_enabled?: boolean;
  permissions?: string;
  /** Subscription plan from ~/.claude/.credentials.json (e.g. 'max', 'pro'). */
  plan?: string;
  /** Rate-limit tier (e.g. 'default_claude_max_5x'). */
  plan_tier?: string;
  /** Local JSONL usage estimate for the last 5h, normalized [0..1]. */
  usage_session_pct?: number;
  /** Local JSONL usage estimate for the last 7d, normalized [0..1]. */
  usage_week_pct?: number;
  usage_calculated_at?: number;
  usage_session_tokens?: number;
  usage_week_tokens?: number;
}

export interface EngineDef {
  engine: string;
  models: string[];
  ctx_windows?: Record<string, number>;
  reasoning_efforts?: string[];
}

export const FALLBACK_ENGINES: EngineDef[] = [
  {
    engine: "claude",
    models: ["sonnet", "opus", "haiku", "opus-1m", "deepseek-v4-pro", "deepseek-v4-flash"],
    ctx_windows: {
      sonnet: 200_000,
      opus: 200_000,
      haiku: 200_000,
      "opus-1m": 1_000_000,
      "deepseek-v4-pro": 128_000,
      "deepseek-v4-flash": 128_000,
    },
    reasoning_efforts: ["low", "medium", "high", "xhigh"],
  },
  {
    engine: "codex",
    models: ["gpt-5.5", "gpt-5.4", "glm-5.1"],
    ctx_windows: { "gpt-5.5": 400_000, "gpt-5.4": 400_000, "glm-5.1": 128_000 },
    reasoning_efforts: ["low", "medium", "high", "xhigh"],
  },
];

export const DEFAULT_REASONING_EFFORTS = ["low", "medium", "high", "xhigh"];

export const AGENT_STATUS_FALLBACK: AgentStatus = {
  engine: "claude",
  model: "sonnet",
  ctx_window: 200_000,
  ctx_used: 0,
  ctx_pct: 0,
  cost_usd: 0,
  permissions: "bypass",
};

export interface UploadAttachment {
  id: string;
  name: string;
  size: number;
  type: string;
  path: string;
  /** true when the backend endpoint was unavailable and we faked the upload client-side */
  pending?: boolean;
}

export interface MessageAttachmentRef {
  id: string;
  name: string;
  type: string;
  path: string;
}

// Backend currently encodes Go's sql.NullString — wire shape is verbose.
// Both shapes are tolerated by `unwrap`.
type NullString = string | null | { String: string; Valid: boolean };
type NullInt = number | null | { Int64: number; Valid: boolean };

interface RawMessage {
  ID: number;
  Channel: string;
  Direction: string;
  JID: NullString;
  Body: NullString;
  MediaType: NullString;
  MediaPath: NullString;
  MediaCaption: NullString;
  LocationLat: NullInt;
  LocationLng: NullInt;
  LocationName: NullString;
  QuotedID: NullInt;
  ReplyTo: NullString;
  TS: number;
  IsRead: number;
  Engine: NullString;
  Model: NullString;
  Activity: NullString;
}

export interface MessageActivityTool {
  id?: string;
  name: string;
  args?: unknown;
  status: "running" | "ok" | "error";
  result_preview?: string;
}

export interface MessageActivity {
  thinking?: string;
  tools?: MessageActivityTool[];
}

export interface RuntimeToolState {
  id?: string;
  name: string;
  args?: unknown;
  status: "running" | "ok" | "error" | "cancelled";
  result_preview?: string;
  started_at?: number;
  finished_at?: number;
  subagent_stats?: Record<string, unknown>;
}

export interface ConversationRuntime {
  scope: string;
  scope_key: string;
  topic?: string;
  engine: string;
  model?: string;
  session_id?: string;
  status: "running" | "done" | "error" | "cancelled" | "interrupted";
  started_at: number;
  updated_at: number;
  finished_at?: number;
  last_error?: string;
  text?: string;
  thinking?: string;
  tools?: RuntimeToolState[];
  result_text?: string;
  last_seq?: number;
}

export interface AgentMessage {
  id: number;
  channel: string;
  direction: "in" | "out";
  body: string;
  ts: number;
  isRead: boolean;
  engine?: string;
  model?: string;
  activity?: MessageActivity;
  media_type?: string;
  media_path?: string;
  media_caption?: string;
}

export interface Project {
  id: number;
  name: string;
  path: string;
  description?: string;
  default_engine: string;
  created_at: number;
  updated_at: number;
  sessions_count?: number;
}

export interface ProjectSession {
  id: number;
  project_id: number;
  name: string;
  session_id: string;
  engine: string;
  model?: string;
  reasoning_effort?: string;
  summary?: string;
  last_active_at?: number;
  created_at: number;
}


export interface ProjectServiceStatus {
  kind: "systemd" | "docker" | "cloudflare-tunnel" | "process" | string;
  description?: string;
  unit?: string;
  container?: string;
  health_cmd?: string;
  hostname?: string;
  target?: string;
  command?: string;
  cwd?: string;
  health_url?: string;
  public_url?: string;
  status: "active" | "stopped" | "failed" | "unknown" | string;
  since?: number;
  cpu_pct?: number;
  mem_mb?: number;
  health_ok: boolean;
  health_error?: string;
}

export interface ProjectMessage {
  id: number;
  scope: string;
  project_id: number;
  project_sess_id: number;
  session_id: string;
  role: "user" | "assistant" | "tool" | "system";
  direction: "in" | "out";
  channel: string;
  body: string;
  activity?: MessageActivity;
  cost_tokens?: number;
  ts: number;
  media_type?: string;
  media_path?: string;
  media_caption?: string;
}

export interface Skill {
  name: string;
  source: string;
  description?: string;
  role_hint?: string;
  version?: string;
  frontmatter?: Record<string, unknown>;
  body: string;
  path: string;
  pulled_at: number;
  updated_at: number;
}

export interface SkillSyncSourceResult {
  source: string;
  path: string;
  discovered: number;
  upserted: number;
  removed: number;
  error?: string;
}

export interface SkillSyncResult {
  sources: SkillSyncSourceResult[];
  started_at: number;
  finished_at: number;
  total_upserted: number;
  total_removed: number;
  errors?: string[];
}

export const skillsApi = {
  async list(source?: string): Promise<Skill[]> {
    const qs = source ? `?source=${encodeURIComponent(source)}` : "";
    const res = await request<{ skills: Skill[] | null }>(`/api/skills${qs}`);
    return res.skills ?? [];
  },
  async sync(): Promise<SkillSyncResult> {
    return request<SkillSyncResult>("/api/skills/sync", { method: "POST" });
  },
};

export interface ProjectTemplate {
  name: string;
  description: string;
  stack?: Record<string, string>;
  agents: Array<{ name: string; role: string; engine: string; model?: string; description?: string }>;
  skills: string[];
  services_initial?: Array<{ kind: string; description?: string; [k: string]: unknown }>;
  claude_md_seed?: string;
  spec_md_seed?: string;
  path: string;
}

export interface ApplyTemplateResult {
  applied: string;
  written_files: string[];
  skipped_files: string[];
  agents_suggested: ProjectTemplate["agents"];
  skills_suggested: string[];
}

export const projectTemplatesApi = {
  async list(): Promise<ProjectTemplate[]> {
    const res = await request<{ templates: ProjectTemplate[] | null }>("/api/project-templates");
    return res.templates ?? [];
  },
  async apply(projectId: number, template: string, overwrite = false): Promise<ApplyTemplateResult> {
    return request<ApplyTemplateResult>(`/api/projects/${projectId}/apply-template`, {
      method: "POST",
      body: JSON.stringify({ template, overwrite }),
    });
  },
};

// ─── BettaTech harness (0.4.0) ─────────────────────────────────────────
export type FeatureStatus = "pending" | "in_progress" | "done" | "blocked";

export interface FeatureItem {
  id: string;
  name: string;
  status: FeatureStatus;
  description?: string;
  depends_on?: string[];
  blocked_reason?: string;
  completed_at?: string;
}

export interface ProjectFeatures {
  exists: boolean;
  path: string;
  version: number;
  updated_at?: string;
  features: FeatureItem[];
}

export interface HarnessFileSnapshot {
  exists: boolean;
  path: string;
  content: string;
  truncated: boolean;
  size: number;
  error?: string;
}

export interface ProjectHarnessState {
  current: HarnessFileSnapshot;
  history: HarnessFileSnapshot;
  checkpoints: HarnessFileSnapshot;
}

export interface Diagram {
  id: number;
  project_id?: number;
  title: string;
  prompt?: string;
  kind: "mermaid" | "html" | string;
  mermaid?: string;
  mermaid_source?: string;
  html_content?: string;
  excalidraw_json: string;
  created_at: number;
  updated_at: number;
}

export interface DiagramPayload {
  title: string;
  prompt?: string;
  kind?: "mermaid" | "html";
  mermaid?: string;
  mermaid_source?: string;
  html_content?: string;
  excalidraw_json?: string;
  project_id?: number;
}

export type DiagramType = "flowchart" | "sequence" | "c4" | "erd" | "mindmap";

function unwrap(v: NullString): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  return v.Valid ? v.String : "";
}

function normalize(raw: RawMessage): AgentMessage {
  const engine = unwrap(raw.Engine);
  const model = unwrap(raw.Model);
  const activityStr = unwrap(raw.Activity);
  let activity: MessageActivity | undefined;
  if (activityStr) {
    try {
      const obj = JSON.parse(activityStr) as MessageActivity;
      if (obj && (obj.thinking || (obj.tools && obj.tools.length > 0))) {
        activity = obj;
      }
    } catch {
      // ignore — not all rows are guaranteed to have valid JSON
    }
  }
  return {
    id: raw.ID,
    channel: raw.Channel,
    direction: raw.Direction === "out" ? "out" : "in",
    body: unwrap(raw.Body),
    ts: raw.TS,
    isRead: raw.IsRead === 1,
    engine: engine || undefined,
    model: model || undefined,
    activity,
    media_type: unwrap(raw.MediaType) || undefined,
    media_path: unwrap(raw.MediaPath) || undefined,
    media_caption: unwrap(raw.MediaCaption) || undefined,
  };
}

class ApiError extends Error {
  status: number;
  constructor(status: number, msg: string) {
    super(msg);
    this.status = status;
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(path, {
    ...init,
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      ...(init.headers ?? {}),
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new ApiError(res.status, text || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  // Some endpoints (logout) return no JSON; guard:
  const ct = res.headers.get("content-type") ?? "";
  if (!ct.includes("application/json")) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  // ─── auth ───────────────────────────────────
  async login(username: string, password: string): Promise<LoginResponse> {
    return request<LoginResponse>("/api/auth/login", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    });
  },

  async totp(
    username: string,
    password: string,
    code: string,
  ): Promise<LoginResponse> {
    return request<LoginResponse>("/api/auth/totp", {
      method: "POST",
      body: JSON.stringify({ username, password, code }),
    });
  },

  async me(): Promise<Me> {
    return request<Me>("/api/auth/me");
  },

  async logout(): Promise<void> {
    await request<void>("/api/auth/logout", { method: "POST" });
  },

  async refresh(): Promise<{ token: string }> {
    return request<{ token: string }>("/api/auth/refresh", { method: "POST" });
  },

  async registerPushToken(token: string): Promise<void> {
    await request<{ ok: boolean }>("/api/push/register", {
      method: "POST",
      body: JSON.stringify({ provider: "fcm", token }),
    });
  },

  // ─── messages ───────────────────────────────
  async listMessages(opts?: {
    before?: number;
    limit?: number;
  }): Promise<AgentMessage[]> {
    const qs = new URLSearchParams();
    if (opts?.before && opts.before > 0) qs.set("before", String(opts.before));
    if (opts?.limit && opts.limit > 0) qs.set("limit", String(opts.limit));
    const path = qs.toString()
      ? `/api/messages?${qs.toString()}`
      : "/api/messages";
    const res = await request<{ messages: RawMessage[] | null }>(path);
    const raw = res.messages ?? [];
    return raw.map(normalize).sort((a, b) => a.ts - b.ts);
  },

  async searchMessages(query: string, limit = 50): Promise<AgentMessage[]> {
    const qs = new URLSearchParams({ q: query, limit: String(limit) });
    const res = await request<{ messages: RawMessage[] | null }>(
      `/api/messages/search?${qs.toString()}`,
    );
    const raw = res.messages ?? [];
    return raw.map(normalize).sort((a, b) => b.ts - a.ts); // search results: newest first
  },

  async sendMessage(
    body: string,
    attachments?: MessageAttachmentRef[],
  ): Promise<SendMessageResponse> {
    const payload: Record<string, unknown> = { body };
    if (attachments && attachments.length > 0) {
      payload.attachments = attachments;
    }
    return request<SendMessageResponse>("/api/messages", {
      method: "POST",
      body: JSON.stringify(payload),
    });
  },

  // ─── agent status ───────────────────────────
  async agentStatus(): Promise<AgentStatus> {
    return request<AgentStatus>("/api/agent/status");
  },

  async listEngines(): Promise<EngineDef[]> {
    const res = await request<{ engines: EngineDef[] | null }>(
      "/api/agent/engines",
    );
    return res.engines ?? [];
  },

  async setEngine(engine: string, model: string): Promise<void> {
    await request<{ ok: boolean; engine: string; model: string }>(
      "/api/agent/engine",
      {
        method: "POST",
        body: JSON.stringify({ engine, model }),
      },
    );
  },

  // Cross-scope cancel — backend identifies the run by (scope, id) and fires
  // its registered context.CancelFunc. Used by the long_running_turn toast.
  /** Snapshot of in-flight turns. Public endpoint. Used by the ChatHUD's
   *  AGENTS · RUNTIME counter so the user sees live concurrency. */
  async getRunsStatus(): Promise<{ runs: Record<string, number>; total: number; pending_restart: boolean }> {
    return request<{ runs: Record<string, number>; total: number; pending_restart: boolean }>("/api/runs");
  },

  async cancelRun(scope: string, id: string): Promise<void> {
    await request<{ cancelled: boolean }>("/api/runs/cancel", {
      method: "POST",
      body: JSON.stringify({ scope, id }),
    });
  },

  // ─── uploads ────────────────────────────────
  async upload(file: File): Promise<UploadAttachment> {
    const fd = new FormData();
    fd.append("file", file);
    const res = await fetch("/api/upload", {
      method: "POST",
      credentials: "include",
      body: fd,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new ApiError(res.status, text || res.statusText);
    }
    return res.json() as Promise<UploadAttachment>;
  },

  uploadUrl(id: string): string {
    return `/api/uploads/${encodeURIComponent(id)}`;
  },

  fileUrl(path: string): string {
    return `/api/file?path=${encodeURIComponent(path)}`;
  },

  async deleteUpload(id: string): Promise<void> {
    const res = await fetch(`/api/uploads/${encodeURIComponent(id)}`, {
      method: "DELETE",
      credentials: "include",
    });
    if (!res.ok && res.status !== 404) {
      const text = await res.text().catch(() => "");
      throw new ApiError(res.status, text || res.statusText);
    }
  },

  // ─── projects ────────────────────────────────
  async listProjects(): Promise<Project[]> {
    const res = await request<{ projects: Project[] | null }>("/api/projects");
    return res.projects ?? [];
  },

  async createProject(payload: {
    name: string;
    path: string;
    description?: string;
    default_engine?: string;
  }): Promise<Project> {
    const res = await request<{ project: Project }>("/api/projects", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return res.project;
  },

  async getProject(
    id: number,
  ): Promise<{ project: Project; sessions: ProjectSession[] }> {
    const res = await request<{
      project: Project;
      sessions: ProjectSession[] | null;
    }>(`/api/projects/${id}`);
    return { project: res.project, sessions: res.sessions ?? [] };
  },

  async listProjectSessions(projectId: number): Promise<ProjectSession[]> {
    const res = await request<{ sessions: ProjectSession[] | null }>(
      `/api/projects/${projectId}/sessions`,
    );
    return res.sessions ?? [];
  },

  async createProjectSession(
    projectId: number,
    payload: { name: string; engine?: string; model?: string; reasoning_effort?: string; summary?: string },
  ): Promise<ProjectSession> {
    const res = await request<{ session: ProjectSession }>(
      `/api/projects/${projectId}/sessions`,
      { method: "POST", body: JSON.stringify(payload) },
    );
    return res.session;
  },

  async deleteProjectSession(projectId: number, sessionId: number): Promise<void> {
    await request<unknown>(
      `/api/projects/${projectId}/sessions/${sessionId}`,
      { method: "DELETE" },
    );
  },

  async listProjectMessages(
    projectId: number,
    sessionId: number,
  ): Promise<ProjectMessage[]> {
    const res = await request<{ messages: (Omit<ProjectMessage, "activity"> & { activity?: string })[] | null }>(
      `/api/projects/${projectId}/sessions/${sessionId}/messages`,
    );
    return (res.messages ?? []).map((m) => {
      let activity: MessageActivity | undefined;
      if (m.activity) {
        try {
          const obj = JSON.parse(m.activity) as MessageActivity;
          if (obj && typeof obj === "object") activity = obj;
        } catch {}
      }
      return { ...m, activity };
    });
  },

  async sendProjectMessage(
    projectId: number,
    sessionId: number,
    body: string,
  ): Promise<{ accepted: boolean; topic: string }> {
    return request<{ accepted: boolean; topic: string }>(
      `/api/projects/${projectId}/sessions/${sessionId}/messages`,
      { method: "POST", body: JSON.stringify({ body }) },
    );
  },

  async getProjectRunStatus(projectId: number, sessionId: number): Promise<{ running: boolean }> {
    return request<{ running: boolean }>(
      `/api/projects/${projectId}/sessions/${sessionId}/run`,
    );
  },

  async getAgentRuntime(): Promise<ConversationRuntime | null> {
    const res = await request<{ run: ConversationRuntime | null }>(`/api/agent/runtime`);
    return res.run ?? null;
  },

  async getProjectRuntime(projectId: number, sessionId: number): Promise<ConversationRuntime | null> {
    const res = await request<{ run: ConversationRuntime | null }>(
      `/api/projects/${projectId}/sessions/${sessionId}/runtime`,
    );
    return res.run ?? null;
  },

  async getProjectFeatures(projectId: number): Promise<ProjectFeatures> {
    return request<ProjectFeatures>(`/api/projects/${projectId}/features`);
  },

  async getProjectHarnessState(projectId: number): Promise<ProjectHarnessState> {
    return request<ProjectHarnessState>(`/api/projects/${projectId}/harness/state`);
  },

  async cancelProjectRun(projectId: number, sessionId: number): Promise<void> {
    await request<unknown>(
      `/api/projects/${projectId}/sessions/${sessionId}/run`,
      { method: "DELETE" },
    );
  },

  async setProjectSessionEngine(projectId: number, sessionId: number, engine: string): Promise<void> {
    await request<unknown>(
      `/api/projects/${projectId}/sessions/${sessionId}/engine`,
      { method: "POST", body: JSON.stringify({ engine }) },
    );
  },

  async setProjectSessionModel(
    projectId: number,
    sessionId: number,
    payload: { model: string; reasoning_effort?: string },
  ): Promise<{ model: string; reasoning_effort: string }> {
    return request<{ model: string; reasoning_effort: string }>(
      `/api/projects/${projectId}/sessions/${sessionId}/model`,
      { method: "POST", body: JSON.stringify(payload) },
    );
  },

  async listProjectServices(projectId: number): Promise<ProjectServiceStatus[]> {
    const res = await request<{ services: ProjectServiceStatus[] | null }>(
      `/api/projects/${projectId}/services`,
    );
    return res.services ?? [];
  },

  async reloadProjectServices(projectId: number): Promise<ProjectServiceStatus[]> {
    const res = await request<{ services: ProjectServiceStatus[] | null }>(
      `/api/projects/${projectId}/services/reload`,
      { method: "POST" },
    );
    return res.services ?? [];
  },

  async projectServiceAction(
    projectId: number,
    index: number,
    action: "start" | "stop" | "restart",
  ): Promise<void> {
    await request<{ ok: boolean }>(
      `/api/projects/${projectId}/services/${index}/${action}`,
      { method: "POST" },
    );
  },

  // ─── diagrams ───────────────────────────────
  async listDiagrams(projectId?: number): Promise<Diagram[]> {
    const qs = new URLSearchParams();
    if (projectId && projectId > 0) qs.set("project_id", String(projectId));
    const path = qs.toString()
      ? `/api/diagrams?${qs.toString()}`
      : "/api/diagrams";
    const res = await request<{ diagrams: Diagram[] | null }>(path);
    return res.diagrams ?? [];
  },

  async getDiagram(id: number): Promise<Diagram> {
    const res = await request<{ diagram: Diagram }>(`/api/diagrams/${id}`);
    return res.diagram;
  },

  async createDiagram(payload: DiagramPayload): Promise<Diagram> {
    const res = await request<{ diagram: Diagram }>("/api/diagrams", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return res.diagram;
  },

  async updateDiagram(id: number, payload: DiagramPayload): Promise<Diagram> {
    const res = await request<{ diagram: Diagram }>(`/api/diagrams/${id}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    });
    return res.diagram;
  },

  async deleteDiagram(id: number): Promise<void> {
    await request<void>(`/api/diagrams/${id}`, { method: "DELETE" });
  },

  async generateDiagram(payload: {
    prompt: string;
    project_id?: number;
    type?: DiagramType;
  }): Promise<{ title: string; mermaid: string }> {
    return request<{ title: string; mermaid: string }>(
      "/api/diagrams/generate",
      {
        method: "POST",
        body: JSON.stringify(payload),
      },
    );
  },

  // ─── system ─────────────────────────────────
  async health(): Promise<{ ok: boolean; ts: number; version?: string; git_commit?: string }> {
    return request<{ ok: boolean; ts: number; version?: string; git_commit?: string }>("/healthz");
  },

  async releases(): Promise<{ content: string; version: string; git_commit: string }> {
    return request<{ content: string; version: string; git_commit: string }>("/api/releases");
  },

  async systemStats(): Promise<SystemStats> {
    return request<SystemStats>("/api/system/stats");
  },

  async systemServices(): Promise<SystemService[]> {
    const res = await request<
      SystemService[] | { services: SystemService[] | null }
    >("/api/system/services");
    if (Array.isArray(res)) return res;
    return res.services ?? [];
  },

  async systemServiceAction(
    name: string,
    action: "start" | "stop" | "restart",
  ): Promise<{ ok: boolean; message?: string }> {
    return request<{ ok: boolean; message?: string }>(
      `/api/system/services/${encodeURIComponent(name)}/${action}`,
      { method: "POST" },
    );
  },

  async systemProcesses(
    top = 10,
    sort: "cpu" | "mem" = "cpu",
  ): Promise<SystemProcess[]> {
    const res = await request<
      SystemProcess[] | { processes: SystemProcess[] | null }
    >(`/api/system/processes?top=${top}&sort=${sort}`);
    if (Array.isArray(res)) return res;
    return res.processes ?? [];
  },

  async systemConnections(): Promise<SystemConnections> {
    return request<SystemConnections>("/api/system/connections");
  },

  async systemCronjobs(): Promise<SystemCronListing> {
    return request<SystemCronListing>("/api/system/cronjobs");
  },

  // ─── usage tracking ─────────────────────────
  async getUsage(params?: UsageQueryParams): Promise<UsageResponse> {
    const qs = new URLSearchParams();
    if (params?.since !== undefined) qs.set("since", String(params.since));
    if (params?.until !== undefined) qs.set("until", String(params.until));
    if (params?.source) qs.set("source", params.source);
    if (params?.model) qs.set("model", params.model);
    if (params?.group_by) qs.set("group_by", params.group_by);
    const path = qs.toString() ? `/api/usage?${qs.toString()}` : "/api/usage";
    return request<UsageResponse>(path);
  },

  async getUsageRealtime(): Promise<RealtimeResponse> {
    return request<RealtimeResponse>("/api/usage/realtime");
  },
};

// ─── usage tracking types ─────────────────────

export interface UsageTotals {
  events: number;
  input_tokens: number;
  output_tokens: number;
  cache_create_tokens: number;
  cache_read_tokens: number;
  cost_usd: number;
}

export interface UsageBucket extends UsageTotals {
  key: string;
}

export interface UsageResponse {
  since: number;
  until: number;
  totals: UsageTotals;
  buckets: UsageBucket[];
}

export interface RealtimeWindow {
  /** 0..100 — el único valor que la API expone (Anthropic devuelve `utilization`, Codex `usedPercent`). */
  percent_used: number;
  /** unix epoch — cuándo se resetea esta ventana. */
  reset_at: number;
  /** Duración de la ventana en minutos (300 = 5h, 10080 = 7d). */
  window_mins?: number;
}

export interface RealtimePerModel {
  model: string;
  percent_used?: number;
}

export interface RealtimeCredits {
  /** Codex devuelve balance como string. */
  balance?: string;
  has_credits?: boolean;
  unlimited?: boolean;
}

export interface RealtimeProvider {
  source: string;
  account?: string;
  plan?: string;
  session?: RealtimeWindow;
  weekly?: RealtimeWindow;
  per_model?: RealtimePerModel[];
  credits?: RealtimeCredits;
  fetched_at: number;
  stale_after: number;
  error?: string;
}

export interface RealtimeResponse {
  claude?: RealtimeProvider;
  codex?: RealtimeProvider;
  fetched_at: number;
}

export interface UsageQueryParams {
  since?: string | number;
  until?: string | number;
  source?: "claude" | "codex";
  model?: string;
  group_by?: "day" | "model" | "source" | "session";
}

// ─── system manager types ─────────────────────
export interface SystemDisk {
  mount: string;
  used_gb: number;
  total_gb: number;
  used_pct: number;
}

export interface SystemStats {
  cpu_pct: number;
  ram_used_gb: number;
  ram_total_gb: number;
  ram_pct?: number;
  disks: SystemDisk[];
  temp_c: number;
  uptime_s: number;
  load_avg: [number, number, number];
  running_agents?: number;
  running_main?: number;
  running_project?: number;
  running_total?: number;
  ws_clients?: number;
}

export interface SystemService {
  name: string;
  state:
    | "active"
    | "inactive"
    | "failed"
    | "activating"
    | "deactivating"
    | string;
  since?: number;
  cpu_pct?: number;
  mem_mb?: number;
  description?: string;
}

export interface SystemProcess {
  pid: number;
  name: string;
  cpu_pct: number;
  mem_mb: number;
}

export interface SystemTunnel {
  name: string;
  state: "up" | "down" | string;
}

export interface SystemConnections {
  wa: "connected" | "disconnected" | "pairing" | string;
  ws_clients: number;
  tunnels: SystemTunnel[];
}

export interface SystemCronJob {
  kind: "user" | "system" | "periodic" | string;
  source: string;
  file?: string;
  line?: number;
  schedule: string;
  user?: string;
  command: string;
}

export interface SystemCronListing {
  jobs: SystemCronJob[];
  warnings?: string[];
  scanned_at: number;
}

// ─── vault (secrets) ──────────────────────────

export interface SecretMeta {
  key: string;
  description?: string;
  scope: string;
  expires_at?: number;
  created_at: number;
  updated_at: number;
  last_accessed_at?: number;
}

export const secretsApi = {
  async list(): Promise<SecretMeta[]> {
    const res = await request<{ secrets: SecretMeta[] | null }>("/api/secrets");
    return res.secrets ?? [];
  },
  async upsert(input: {
    key: string;
    value: string;
    description?: string;
    scope?: string;
    expires_at?: number;
  }): Promise<void> {
    await request("/api/secrets", {
      method: "POST",
      body: JSON.stringify(input),
    });
  },
  async reveal(key: string): Promise<string> {
    const res = await request<{ key: string; value: string; ts: number }>(
      `/api/secrets/${encodeURIComponent(key)}/reveal`,
    );
    return res.value;
  },
  async delete(key: string): Promise<void> {
    await request(`/api/secrets/${encodeURIComponent(key)}`, {
      method: "DELETE",
    });
  },
};

// ─── topics ───────────────────────────────────

export interface Topic {
  id: number;
  name: string;
  description?: string;
  keywords?: string[];
  project_id?: number;
  session_id?: string;
  engine: string;
  is_default: boolean;
  last_active_at?: number;
  created_at: number;
}

export interface TopicState {
  topic_id: number;
  headline?: string;
  active_issues?: string[];
  recent_decisions?: string[];
  pending?: string[];
  next_action_hint?: string;
  last_event_at?: number;
  updated_at: number;
}

export const topicsApi = {
  async list(): Promise<Topic[]> {
    const res = await request<{ topics: Topic[] | null }>("/api/topics");
    return res.topics ?? [];
  },
  async create(
    name: string,
    description?: string,
    keywords?: string[],
  ): Promise<{ id: number }> {
    return request<{ id: number; name: string }>("/api/topics", {
      method: "POST",
      body: JSON.stringify({ name, description, keywords }),
    });
  },
  async getState(id: number): Promise<TopicState> {
    return request<TopicState>(`/api/topics/${id}/state`);
  },
  async updateState(id: number, patch: Partial<TopicState>): Promise<void> {
    await request(`/api/topics/${id}/state`, {
      method: "POST",
      body: JSON.stringify(patch),
    });
  },
};

// ─── subagents ────────────────────────────────

export interface Subagent {
  id: number;
  parent_session_id: string;
  parent_scope: "main" | "topic" | "project" | "agent" | string;
  parent_topic_id?: number;
  parent_project_session_id?: number;
  agent_type?: string;
  description?: string;
  prompt?: string;
  result?: string;
  status: "running" | "ok" | "error" | "cancelled";
  started_at: number;
  finished_at?: number;
  cost_tokens: number;
  tools_used?: string;
  worktree_path?: string;
}

export const subagentsApi = {
  async list(status?: string, limit = 50): Promise<Subagent[]> {
    const qs = new URLSearchParams();
    if (status) qs.set("status", status);
    qs.set("limit", String(limit));
    const res = await request<{ subagents: Subagent[] | null }>(
      `/api/subagents?${qs.toString()}`,
    );
    return res.subagents ?? [];
  },
  async get(id: number): Promise<Subagent> {
    return request<Subagent>(`/api/subagents/${id}`);
  },
};

// ─── websocket helper ─────────────────────────
export function wsUrl(path: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  // dev: vite proxy at :5173 forwards /ws → :8093 with ws upgrade
  // prod: backend serves frontend & ws on same origin
  return `${proto}//${window.location.host}${path}`;
}

export { ApiError };
