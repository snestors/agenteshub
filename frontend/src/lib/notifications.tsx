import * as React from "react";
import { useLocation } from "react-router-dom";
import { Bot, AlertTriangle, Info, X } from "lucide-react";
import { wsClient } from "@/lib/wsClient";

export interface Notification {
  id: string;
  kind: string;
  severity: "info" | "warn" | "error";
  title: string;
  body?: string;
  context?: Record<string, unknown>;
  ts: number;
  /** local-only flag: set when the user has marked the toast as seen */
  read?: boolean;
}

interface NotificationsState {
  items: Notification[];
  unreadByKindPrefix: (prefix: string) => number;
  markAllRead: () => void;
  dismiss: (id: string) => void;
  push: (n: Notification) => void;
}

const Ctx = React.createContext<NotificationsState | null>(null);

const MAX_KEEP = 50;
const TOAST_TTL_MS = 6000;

function parsePayload(payload: unknown): Notification | null {
  if (typeof payload === "string") {
    try {
      return JSON.parse(payload) as Notification;
    } catch {
      return null;
    }
  }
  if (payload && typeof payload === "object") return payload as Notification;
  return null;
}

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<Notification[]>([]);

  const push = React.useCallback((n: Notification) => {
    setItems((curr) => {
      const next = [n, ...curr].slice(0, MAX_KEEP);
      return next;
    });
  }, []);

  const dismiss = React.useCallback((id: string) => {
    setItems((curr) => curr.filter((i) => i.id !== id));
  }, []);

  const markAllRead = React.useCallback(() => {
    setItems((curr) => curr.map((i) => ({ ...i, read: true })));
  }, []);

  const unreadByKindPrefix = React.useCallback(
    (prefix: string) =>
      items.reduce((acc, i) => (!i.read && i.kind.startsWith(prefix) ? acc + 1 : acc), 0),
    [items]
  );

  // Subscribe globally to the 'notifications' topic.
  React.useEffect(() => {
    const off = wsClient.subscribe("notifications", (env) => {
      if (env.type !== "notification") return;
      const n = parsePayload(env.payload);
      if (!n || !n.id) return;
      push(n);
    });
    return off;
  }, [push]);

  const value = React.useMemo<NotificationsState>(
    () => ({ items, unreadByKindPrefix, markAllRead, dismiss, push }),
    [items, unreadByKindPrefix, markAllRead, dismiss, push]
  );

  return (
    <Ctx.Provider value={value}>
      {children}
      <RoutedToastStack items={items} dismiss={dismiss} />
    </Ctx.Provider>
  );
}

// shouldToast hides the toast when the user is already on the screen that
// would naturally surface that event. The notification is still kept in the
// store so the badge counter is honest.
function shouldToast(kind: string, pathname: string): boolean {
  if (kind.startsWith("main_turn") && pathname === "/") return false;
  if (kind.startsWith("agent_run") && pathname.startsWith("/agents")) return false;
  if (kind.startsWith("project_turn") && pathname.startsWith("/projects")) return false;
  return true;
}

export function useNotifications(): NotificationsState {
  const v = React.useContext(Ctx);
  if (!v) throw new Error("useNotifications must be used inside NotificationProvider");
  return v;
}

// ─── Toast stack ─────────────────────────────────────────────

function RoutedToastStack({
  items,
  dismiss,
}: {
  items: Notification[];
  dismiss: (id: string) => void;
}) {
  const { pathname } = useLocation();
  const [now, setNow] = React.useState(Date.now());
  React.useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  const visible = items
    .filter((i) => !i.read && now - i.ts * 1000 < TOAST_TTL_MS && shouldToast(i.kind, pathname))
    .slice(0, 4);

  if (visible.length === 0) return null;
  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm pointer-events-none">
      {visible.map((n) => (
        <Toast key={n.id} n={n} onClose={() => dismiss(n.id)} />
      ))}
    </div>
  );
}

function Toast({ n, onClose }: { n: Notification; onClose: () => void }) {
  const accent =
    n.severity === "error"
      ? "var(--color-danger)"
      : n.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  const Icon = n.severity === "error" ? AlertTriangle : n.kind.startsWith("agent_run") ? Bot : Info;
  return (
    <div
      className="pointer-events-auto clip-tag flex gap-3 px-3 py-2 backdrop-blur-sm border"
      style={{
        background: "rgba(10, 15, 36, 0.90)",
        borderColor: accent,
        boxShadow: `0 0 14px ${accent}40`,
      }}
    >
      <Icon size={16} strokeWidth={1.6} style={{ color: accent, marginTop: 2 }} />
      <div className="flex-1 min-w-0">
        <div className="font-mono text-[11px] uppercase tracking-hud-tight" style={{ color: accent }}>
          {n.title}
        </div>
        {n.body && (
          <div className="font-mono text-[11px] text-[var(--color-fg)] mt-1 break-words">
            {n.body}
          </div>
        )}
      </div>
      <button
        onClick={onClose}
        className="text-[var(--color-dim)] hover:text-[var(--color-fg)] transition-colors p-0.5 cursor-pointer self-start"
        aria-label="cerrar"
      >
        <X size={12} strokeWidth={1.8} />
      </button>
    </div>
  );
}
