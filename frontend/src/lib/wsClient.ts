// Module-level singleton WebSocket client for the unified /ws endpoint.
//
// Protocol:
//   CLIENT → SERVER
//     { action: "subscribe",   topic: "agent" }
//     { action: "subscribe",   topic: "system" }
//     { action: "unsubscribe", topic: "agent" }
//     { action: "send_message", id: "<corr-id>", body: "...", attachments: [...] }
//     { action: "ping" }
//
//   SERVER → CLIENT (envelope)
//     { type: "message" | "stream" | "stats" | "status" | "ack", topic, payload, ts }
//
// Lifecycle:
//   - One persistent connection from import time until page unload.
//   - Exponential backoff on reconnects: [1s, 2s, 5s, 10s, 30s].
//   - After `fallbackAfter` failed attempts (default 3) status flips to "fallback"
//     so consumers can switch to polling.
//   - When the socket reopens after fallback, all active topic subscriptions
//     are re-sent automatically.
//
// Topic listeners:
//   - subscribe(topic, listener) registers a listener and (if it's the first
//     listener for that topic) sends `{action:"subscribe", topic}` to the
//     server. Returns an unsubscribe fn that decrements the listener set;
//     when the set hits zero, sends `{action:"unsubscribe", topic}`.
//   - All envelopes whose `topic` field matches are dispatched to the topic's
//     listeners. Envelopes without a topic (or with an unknown one) are dropped.

import { wsUrl } from "./api";

export type WsStatus = "connecting" | "open" | "closed" | "fallback";

export interface WsEnvelope {
  type: string;
  topic?: string;
  payload?: unknown;
  ts?: number;
}

export type WsListener = (envelope: WsEnvelope) => void;
export type WsStatusListener = (status: WsStatus) => void;

export interface WsAckPayload<T = unknown> {
  id: string;
  ok: boolean;
  error?: string;
  result?: T;
}

export interface WsClientAPI {
  /** Subscribe to a topic. Returns an unsubscribe function. */
  subscribe(topic: string, listener: WsListener): () => void;
  /** Send a low-level action to the server. Buffers nothing; drops if not open. */
  send(action: string, payload?: object): void;
  /** Send an RPC action and wait for a correlated ack envelope. */
  request<T = unknown>(action: string, payload?: object, timeoutMs?: number): Promise<T>;
  /** Current connection status. */
  status(): WsStatus;
  /** Subscribe to status changes. Returns an unsubscribe function. */
  onStatusChange(cb: WsStatusListener): () => void;
}

const DEFAULT_BACKOFF = [1000, 2000, 5000, 10000, 30000];
const DEFAULT_FALLBACK_AFTER = 3;

interface CreateOpts {
  backoff?: number[];
  fallbackAfter?: number;
}

function createWsClient(path: string, opts: CreateOpts = {}): WsClientAPI {
  const backoff = opts.backoff ?? DEFAULT_BACKOFF;
  const fallbackAfter = opts.fallbackAfter ?? DEFAULT_FALLBACK_AFTER;

  let ws: WebSocket | null = null;
  let status: WsStatus = "connecting";
  let attempt = 0;
  let fallbackFired = false;
  let reconnectTimer: number | null = null;
  let stopped = false;

  // topic → set of listeners
  const topicListeners = new Map<string, Set<WsListener>>();
  // status listeners
  const statusListeners = new Set<WsStatusListener>();
  // correlation id → pending request
  const pendingRequests = new Map<
    string,
    {
      resolve: (value: unknown) => void;
      reject: (reason?: unknown) => void;
      timer: number;
    }
  >();

  function setStatus(next: WsStatus) {
    if (status === next) return;
    status = next;
    for (const cb of statusListeners) {
      try {
        cb(next);
      } catch {
        /* swallow */
      }
    }
  }

  function clearReconnect() {
    if (reconnectTimer !== null) {
      window.clearTimeout(reconnectTimer);
      reconnectTimer = null;
    }
  }

  function scheduleReconnect() {
    if (stopped) return;
    const idx = Math.min(attempt, backoff.length - 1);
    const delay = backoff[idx];
    attempt += 1;
    if (attempt >= fallbackAfter && !fallbackFired) {
      fallbackFired = true;
      setStatus("fallback");
    }
    reconnectTimer = window.setTimeout(connect, delay);
  }

  function rawSend(obj: object): boolean {
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    try {
      ws.send(JSON.stringify(obj));
      return true;
    } catch {
      return false;
    }
  }

  function resubscribeAll() {
    for (const topic of topicListeners.keys()) {
      rawSend({ action: "subscribe", topic });
    }
  }

  function dispatchEnvelope(env: WsEnvelope) {
    if (!env.topic) return;
    const set = topicListeners.get(env.topic);
    if (!set || set.size === 0) return;
    // copy to tolerate listeners that unsubscribe during dispatch
    for (const listener of Array.from(set)) {
      try {
        listener(env);
      } catch {
        /* swallow listener errors */
      }
    }
  }

  function parseAckPayload(payload: unknown): WsAckPayload | null {
    if (!payload) return null;
    if (typeof payload === "string") {
      try {
        const obj = JSON.parse(payload);
        return obj && typeof obj === "object" ? (obj as WsAckPayload) : null;
      } catch {
        return null;
      }
    }
    if (typeof payload === "object") return payload as WsAckPayload;
    return null;
  }

  function dispatchAck(env: WsEnvelope): boolean {
    if (env.type !== "ack") return false;
    const ack = parseAckPayload(env.payload);
    if (!ack?.id) return false;
    const pending = pendingRequests.get(ack.id);
    if (!pending) return false;
    pendingRequests.delete(ack.id);
    window.clearTimeout(pending.timer);
    if (ack.ok) {
      pending.resolve(ack.result);
    } else {
      pending.reject(new Error(ack.error || "ws request failed"));
    }
    return true;
  }

  function connect() {
    if (stopped) return;
    clearReconnect();
    if (status !== "fallback") setStatus("connecting");

    let socket: WebSocket;
    try {
      socket = new WebSocket(wsUrl(path));
    } catch {
      scheduleReconnect();
      return;
    }
    ws = socket;

    socket.onopen = () => {
      if (ws !== socket) return; // stale
      attempt = 0;
      const wasFallback = fallbackFired;
      fallbackFired = false;
      setStatus("open");
      // re-subscribe to all topics on (re)connect
      resubscribeAll();
      // when recovering from fallback, listeners using onStatusChange will see
      // the transition fallback → open and can stop their polling.
      void wasFallback;
    };

    socket.onmessage = (ev) => {
      if (ws !== socket) return;
      if (typeof ev.data !== "string") return;
      let parsed: unknown;
      try {
        parsed = JSON.parse(ev.data);
      } catch {
        return;
      }
      if (!parsed || typeof parsed !== "object") return;
      const env = parsed as WsEnvelope;
      if (dispatchAck(env)) return;
      dispatchEnvelope(env);
    };

    socket.onerror = () => {
      // browsers fire onerror before onclose; let onclose drive reconnect
    };

    socket.onclose = () => {
      if (ws === socket) ws = null;
      if (stopped) return;
      if (status !== "fallback") setStatus("closed");
      scheduleReconnect();
    };
  }

  // Kick off the initial connection. Wrapped in try so SSR or test
  // environments without WebSocket don't crash module evaluation.
  try {
    if (typeof window !== "undefined" && typeof WebSocket !== "undefined") {
      connect();
      // best-effort cleanup on page unload
      window.addEventListener("beforeunload", () => {
        stopped = true;
        clearReconnect();
        if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
          try {
            ws.close();
          } catch {
            /* noop */
          }
        }
      });
    }
  } catch {
    /* noop */
  }

  function subscribe(topic: string, listener: WsListener): () => void {
    let set = topicListeners.get(topic);
    const isFirst = !set || set.size === 0;
    if (!set) {
      set = new Set();
      topicListeners.set(topic, set);
    }
    set.add(listener);
    if (isFirst) {
      // if not open yet, resubscribeAll() on open will pick this up
      rawSend({ action: "subscribe", topic });
    }
    return () => {
      const s = topicListeners.get(topic);
      if (!s) return;
      s.delete(listener);
      if (s.size === 0) {
        topicListeners.delete(topic);
        rawSend({ action: "unsubscribe", topic });
      }
    };
  }

  function send(action: string, payload?: object) {
    const obj: Record<string, unknown> = { action };
    if (payload && typeof payload === "object") {
      Object.assign(obj, payload);
    }
    rawSend(obj);
  }

  function newRequestID(): string {
    const cryptoObj = globalThis.crypto;
    if (cryptoObj && typeof cryptoObj.randomUUID === "function") {
      return cryptoObj.randomUUID();
    }
    return `req-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  }

  function request<T = unknown>(
    action: string,
    payload: object = {},
    timeoutMs = 10_000
  ): Promise<T> {
    const id = newRequestID();
    const obj: Record<string, unknown> = { action, id };
    if (payload && typeof payload === "object") {
      Object.assign(obj, payload);
    }
    return new Promise<T>((resolve, reject) => {
      const timer = window.setTimeout(() => {
        pendingRequests.delete(id);
        reject(new Error(`timeout esperando ack de ${action}`));
      }, timeoutMs);
      pendingRequests.set(id, {
        resolve: (value) => resolve(value as T),
        reject,
        timer,
      });
      if (!rawSend(obj)) {
        pendingRequests.delete(id);
        window.clearTimeout(timer);
        reject(new Error("websocket no conectado"));
      }
    });
  }

  function onStatusChange(cb: WsStatusListener): () => void {
    statusListeners.add(cb);
    return () => {
      statusListeners.delete(cb);
    };
  }

  return {
    subscribe,
    send,
    request,
    status: () => status,
    onStatusChange,
  };
}

export const wsClient: WsClientAPI = createWsClient("/ws");

// Exported for tests / advanced use cases (e.g. spinning up a second client
// against a different path). Application code should use `wsClient`.
export { createWsClient };
