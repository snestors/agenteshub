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
  cost_usd: number;
  session_id?: string;
  wa_enabled?: boolean;
  permissions: string;
}

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
}

function unwrap(v: NullString): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  return v.Valid ? v.String : "";
}

function normalize(raw: RawMessage): AgentMessage {
  const engine = unwrap(raw.Engine);
  const model = unwrap(raw.Model);
  return {
    id: raw.ID,
    channel: raw.Channel,
    direction: raw.Direction === "out" ? "out" : "in",
    body: unwrap(raw.Body),
    ts: raw.TS,
    isRead: raw.IsRead === 1,
    engine: engine || undefined,
    model: model || undefined,
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
