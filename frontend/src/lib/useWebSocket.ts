import * as React from "react";

export type WsStatus = "connecting" | "open" | "closed" | "fallback";

interface UseWebSocketOptions {
  /** Full URL (e.g. ws://host/ws/agent). Pass `null` to keep the socket closed. */
  url: string | null;
  /** Called for each parsed message. */
  onMessage: (data: unknown, raw: MessageEvent) => void;
  /** Backoff steps in ms. Loops on the last value once exhausted. */
  backoff?: number[];
  /** After this many *consecutive* failed attempts, flip to "fallback". */
  fallbackAfter?: number;
  /** When fallback fires, useful for the consumer to start polling. */
  onFallback?: () => void;
  /** When the socket recovers from fallback, useful to stop polling. */
  onReconnected?: () => void;
}

const DEFAULT_BACKOFF = [1000, 2000, 5000, 10000, 30000];

/**
 * Resilient WebSocket hook with exponential-ish backoff and
 * a "fallback" status the consumer can use to switch to polling.
 *
 * - Returns the current `status` and a `send()` helper.
 * - Only one socket is alive at a time.
 * - If the URL changes (or becomes null) the socket is closed cleanly.
 * - Once the socket connects, the failure counter resets so we don't
 *   permanently stay in fallback after a flaky network.
 */
export function useWebSocket({
  url,
  onMessage,
  backoff = DEFAULT_BACKOFF,
  fallbackAfter = 3,
  onFallback,
  onReconnected,
}: UseWebSocketOptions) {
  const [status, setStatus] = React.useState<WsStatus>("connecting");
  const wsRef = React.useRef<WebSocket | null>(null);
  const timerRef = React.useRef<number | null>(null);
  const attemptRef = React.useRef(0);
  const fallbackFiredRef = React.useRef(false);
  // refs for callbacks so the effect doesn't re-run on every render
  const onMessageRef = React.useRef(onMessage);
  const onFallbackRef = React.useRef(onFallback);
  const onReconnectedRef = React.useRef(onReconnected);
  React.useEffect(() => {
    onMessageRef.current = onMessage;
    onFallbackRef.current = onFallback;
    onReconnectedRef.current = onReconnected;
  });

  React.useEffect(() => {
    if (!url) {
      setStatus("closed");
      return;
    }

    let cancelled = false;

    function clearTimer() {
      if (timerRef.current !== null) {
        window.clearTimeout(timerRef.current);
        timerRef.current = null;
      }
    }

    function scheduleReconnect() {
      if (cancelled) return;
      const idx = Math.min(attemptRef.current, backoff.length - 1);
      const delay = backoff[idx];
      attemptRef.current += 1;
      if (
        attemptRef.current >= fallbackAfter &&
        !fallbackFiredRef.current
      ) {
        fallbackFiredRef.current = true;
        setStatus("fallback");
        onFallbackRef.current?.();
      }
      timerRef.current = window.setTimeout(connect, delay);
    }

    function connect() {
      if (cancelled || !url) return;
      try {
        setStatus((prev) => (prev === "fallback" ? "fallback" : "connecting"));
        const ws = new WebSocket(url);
        wsRef.current = ws;

        ws.onopen = () => {
          if (cancelled) return;
          attemptRef.current = 0;
          setStatus("open");
          if (fallbackFiredRef.current) {
            fallbackFiredRef.current = false;
            onReconnectedRef.current?.();
          }
        };

        ws.onmessage = (ev) => {
          if (cancelled) return;
          let parsed: unknown = ev.data;
          if (typeof ev.data === "string") {
            try {
              parsed = JSON.parse(ev.data);
            } catch {
              parsed = ev.data;
            }
          }
          onMessageRef.current(parsed, ev);
        };

        ws.onerror = () => {
          // browsers fire onerror before onclose; we let onclose drive reconnect
        };

        ws.onclose = () => {
          if (cancelled) return;
          wsRef.current = null;
          if (status !== "fallback") setStatus("closed");
          scheduleReconnect();
        };
      } catch {
        scheduleReconnect();
      }
    }

    // reset state when url changes
    attemptRef.current = 0;
    fallbackFiredRef.current = false;
    connect();

    return () => {
      cancelled = true;
      clearTimer();
      const ws = wsRef.current;
      wsRef.current = null;
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
        try {
          ws.close();
        } catch {
          /* noop */
        }
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [url, fallbackAfter]);

  const send = React.useCallback((data: string | object) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) return false;
    const payload = typeof data === "string" ? data : JSON.stringify(data);
    ws.send(payload);
    return true;
  }, []);

  return { status, send };
}
