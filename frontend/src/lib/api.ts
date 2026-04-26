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
}

export interface AgentMessage {
  id: number;
  channel: string;
  direction: "in" | "out";
  body: string;
  ts: number;
  isRead: boolean;
}

function unwrap(v: NullString): string {
  if (v == null) return "";
  if (typeof v === "string") return v;
  return v.Valid ? v.String : "";
}

function normalize(raw: RawMessage): AgentMessage {
  return {
    id: raw.ID,
    channel: raw.Channel,
    direction: raw.Direction === "out" ? "out" : "in",
    body: unwrap(raw.Body),
    ts: raw.TS,
    isRead: raw.IsRead === 1,
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

  async sendMessage(body: string): Promise<SendMessageResponse> {
    return request<SendMessageResponse>("/api/messages", {
      method: "POST",
      body: JSON.stringify({ body }),
    });
  },

  // ─── system ─────────────────────────────────
  async health(): Promise<{ ok: boolean; ts: number }> {
    return request<{ ok: boolean; ts: number }>("/healthz");
  },
};

export { ApiError };
