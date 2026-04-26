// Module-level singleton WebSocket client for the unified /ws endpoint.
//
// Protocol:
//   CLIENT → SERVER
//     { action: "subscribe",   topic: "agent" }
//     { action: "subscribe",   topic: "system" }
//     { action: "unsubscribe", topic: "agent" }
//     { action: "ping" }
//
//   SERVER → CLIENT (envelope)
//     { type: "message" | "stream" | "stats" | "status", topic, payload, ts }
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

export interface WsClientAPI {
  /** Subscribe to a topic. Returns an unsubscribe function. */
  subscribe(topic: string, listener: WsListener): () => void;
  /** Send a low-level action to the server. Buffers nothing; drops if not open. */
  send(action: string, payload?: object): void;
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
      dispatchEnvelope(parsed as WsEnvelope);
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

  function onStatusChange(cb: WsStatusListener): () => void {
    statusListeners.add(cb);
    return () => {
      statusListeners.delete(cb);
    };
  }

  return {
    subscribe,
    send,
    status: () => status,
    onStatusChange,
  };
}

export const wsClient: WsClientAPI = createWsClient("/ws");

// Exported for tests / advanced use cases (e.g. spinning up a second client
// against a different path). Application code should use `wsClient`.
export { createWsClient };
