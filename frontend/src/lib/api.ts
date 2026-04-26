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
}

export const FALLBACK_ENGINES: EngineDef[] = [
  {
    engine: "claude",
    models: ["sonnet", "opus", "haiku"],
    ctx_windows: { sonnet: 200_000, opus: 200_000, haiku: 200_000 },
  },
  {
    engine: "codex",
    models: ["gpt-5.5"],
    ctx_windows: { "gpt-5.5": 400_000 },
  },
  {
    engine: "ollama",
    models: ["gemma:2b"],
    ctx_windows: { "gemma:2b": 8_192 },
  },
];

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
  summary?: string;
  last_active_at?: number;
  created_at: number;
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
  cost_tokens?: number;
  ts: number;
}

export interface MiniAgent {
  id: number;
  name: string;
  description?: string;
  system_prompt?: string;
  engine: string;
  enabled: boolean;
  project_id?: number;
  created_at: number;
  updated_at: number;
  next_run?: number;
  schedules_count?: number;
  runs_24h?: number;
}

export interface AgentSchedule {
  id: number;
  agent_id: number;
  cron_expr: string;
  prompt_template: string;
  notify_target: string;
  enabled: boolean;
  last_run_at?: number;
  next_run: number;
}

export interface AgentRun {
  id: number;
  agent_id: number;
  schedule_id?: number;
  trigger: string;
  started_at: number;
  finished_at?: number;
  status: "running" | "ok" | "error" | "cancelled" | string;
  prompt: string;
  result?: string;
  tools_used?: string;
  cost_tokens: number;
  error?: string;
}

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

  async totp(username: string, password: string, code: string): Promise<LoginResponse> {
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

  // ─── messages ───────────────────────────────
  async listMessages(): Promise<AgentMessage[]> {
    const res = await request<{ messages: RawMessage[] | null }>("/api/messages");
    const raw = res.messages ?? [];
    return raw.map(normalize).sort((a, b) => a.ts - b.ts);
  },

  async sendMessage(
    body: string,
    attachments?: MessageAttachmentRef[]
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
    const res = await request<{ engines: EngineDef[] | null }>("/api/agent/engines");
    return res.engines ?? [];
  },

  async setEngine(engine: string, model: string): Promise<void> {
    await request<{ ok: boolean; engine: string; model: string }>("/api/agent/engine", {
      method: "POST",
      body: JSON.stringify({ engine, model }),
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

  async getProject(id: number): Promise<{ project: Project; sessions: ProjectSession[] }> {
    const res = await request<{
      project: Project;
      sessions: ProjectSession[] | null;
    }>(`/api/projects/${id}`);
    return { project: res.project, sessions: res.sessions ?? [] };
  },

  async listProjectSessions(projectId: number): Promise<ProjectSession[]> {
    const res = await request<{ sessions: ProjectSession[] | null }>(
      `/api/projects/${projectId}/sessions`
    );
    return res.sessions ?? [];
  },

  async createProjectSession(
    projectId: number,
    payload: { name: string; engine?: string; summary?: string }
  ): Promise<ProjectSession> {
    const res = await request<{ session: ProjectSession }>(
      `/api/projects/${projectId}/sessions`,
      { method: "POST", body: JSON.stringify(payload) }
    );
    return res.session;
  },

  async listProjectMessages(
    projectId: number,
    sessionId: number
  ): Promise<ProjectMessage[]> {
    const res = await request<{ messages: ProjectMessage[] | null }>(
      `/api/projects/${projectId}/sessions/${sessionId}/messages`
    );
    return res.messages ?? [];
  },

  async sendProjectMessage(
    projectId: number,
    sessionId: number,
    body: string
  ): Promise<{ accepted: boolean; topic: string }> {
    return request<{ accepted: boolean; topic: string }>(
      `/api/projects/${projectId}/sessions/${sessionId}/messages`,
      { method: "POST", body: JSON.stringify({ body }) }
    );
  },

  // ─── mini-agents ─────────────────────────────
  async listAgents(): Promise<MiniAgent[]> {
    const res = await request<{ agents: MiniAgent[] | null }>("/api/agents");
    return res.agents ?? [];
  },

  async createAgent(payload: {
    name: string;
    description?: string;
    system_prompt: string;
    engine?: string;
    project_id?: number;
  }): Promise<MiniAgent> {
    const res = await request<{ agent: MiniAgent }>("/api/agents", {
      method: "POST",
      body: JSON.stringify(payload),
    });
    return res.agent;
  },

  async getAgent(id: number): Promise<{
    agent: MiniAgent;
    schedules: AgentSchedule[];
    runs: AgentRun[];
  }> {
    const res = await request<{
      agent: MiniAgent;
      schedules: AgentSchedule[] | null;
      runs: AgentRun[] | null;
    }>(`/api/agents/${id}`);
    return {
      agent: res.agent,
      schedules: res.schedules ?? [],
      runs: res.runs ?? [],
    };
  },

  async setAgentEnabled(id: number, enabled: boolean): Promise<void> {
    await request(`/api/agents/${id}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    });
  },

  async runAgentNow(id: number, prompt?: string): Promise<{ run_id: number; topic: string }> {
    return request<{ run_id: number; topic: string }>(`/api/agents/${id}/run`, {
      method: "POST",
      body: JSON.stringify({ prompt: prompt ?? "" }),
    });
  },

  async listAgentRuns(id: number, limit = 50): Promise<AgentRun[]> {
    const res = await request<{ runs: AgentRun[] | null }>(
      `/api/agents/${id}/runs?limit=${limit}`
    );
    return res.runs ?? [];
  },

  async addAgentSchedule(
    id: number,
    payload: { cron_expr: string; prompt_template: string; notify_target?: string }
  ): Promise<AgentSchedule> {
    const res = await request<{ schedule: AgentSchedule }>(
      `/api/agents/${id}/schedules`,
      { method: "POST", body: JSON.stringify(payload) }
    );
    return res.schedule;
  },

  async setAgentScheduleEnabled(
    agentId: number,
    scheduleId: number,
    enabled: boolean
  ): Promise<void> {
    await request(`/api/agents/${agentId}/schedules/${scheduleId}/enabled`, {
      method: "POST",
      body: JSON.stringify({ enabled }),
    });
  },

  async deleteAgentSchedule(agentId: number, scheduleId: number): Promise<void> {
    await request(`/api/agents/${agentId}/schedules/${scheduleId}`, {
      method: "DELETE",
    });
  },

  // ─── system ─────────────────────────────────
  async health(): Promise<{ ok: boolean; ts: number }> {
    return request<{ ok: boolean; ts: number }>("/healthz");
  },

  async systemStats(): Promise<SystemStats> {
    return request<SystemStats>("/api/system/stats");
  },

  async systemServices(): Promise<SystemService[]> {
    const res = await request<SystemService[] | { services: SystemService[] | null }>(
      "/api/system/services"
    );
    if (Array.isArray(res)) return res;
    return res.services ?? [];
  },

  async systemServiceAction(
    name: string,
    action: "start" | "stop" | "restart"
  ): Promise<{ ok: boolean; message?: string }> {
    return request<{ ok: boolean; message?: string }>(
      `/api/system/services/${encodeURIComponent(name)}/${action}`,
      { method: "POST" }
    );
  },

  async systemProcesses(top = 10, sort: "cpu" | "mem" = "cpu"): Promise<SystemProcess[]> {
    const res = await request<SystemProcess[] | { processes: SystemProcess[] | null }>(
      `/api/system/processes?top=${top}&sort=${sort}`
    );
    if (Array.isArray(res)) return res;
    return res.processes ?? [];
  },

  async systemConnections(): Promise<SystemConnections> {
    return request<SystemConnections>("/api/system/connections");
  },
};

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
  state: "active" | "inactive" | "failed" | "activating" | "deactivating" | string;
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

// ─── websocket helper ─────────────────────────
export function wsUrl(path: string): string {
  const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
  // dev: vite proxy at :5173 forwards /ws → :8093 with ws upgrade
  // prod: backend serves frontend & ws on same origin
  return `${proto}//${window.location.host}${path}`;
}

export { ApiError };
