// NotifCenter — bell + drawer + toast (Claude Design handoff Sprint B).
//
// Exports:
//   <NotifProvider>   wrap once in AppShell so the WS subscription + state
//                     are shared across the app
//   useNotifCenter()  hook to read state + actions
//   <NotifBell>       button to drop into Topbar
//
// The provider is responsible for:
//   1. initial REST fetch of /api/notifications
//   2. WS subscription to topic="notifications"; every incoming envelope
//      prepends the new notif to items, bumps unread, and pops a toast
//   3. drawer open/close state
//   4. mark-read / mark-all-read REST roundtrips
//
// Animations come from index.css: anim-bell-shake, anim-ring-pulse,
// anim-drawer-in, anim-toast-pop, anim-fade-in-up.

import * as React from "react";
import { Bell, X } from "lucide-react";
import { wsClient } from "@/lib/wsClient";

const TOAST_TTL_MS = 5000;
const NOTIF_LIST_LIMIT = 50;

export interface NotifItem {
  id: string;
  kind: string;
  severity: "info" | "warn" | "error" | string;
  title: string;
  body?: string;
  context?: Record<string, unknown>;
  ts: number;
  read: boolean;
}

interface NotifCenterCtx {
  items: NotifItem[];
  unread: number;
  isOpen: boolean;
  toast: NotifItem | null;
  open: () => void;
  close: () => void;
  toggle: () => void;
  markRead: (id: string) => Promise<void>;
  markAllRead: () => Promise<void>;
  dismissToast: () => void;
  /** Spike for the bell shake animation; flips with each new incoming notif. */
  shakeKey: number;
}

const Ctx = React.createContext<NotifCenterCtx | null>(null);

export function useNotifCenter(): NotifCenterCtx {
  const v = React.useContext(Ctx);
  if (!v) throw new Error("useNotifCenter outside NotifProvider");
  return v;
}

export function NotifProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<NotifItem[]>([]);
  const [unread, setUnread] = React.useState(0);
  const [isOpen, setIsOpen] = React.useState(false);
  const [toast, setToast] = React.useState<NotifItem | null>(null);
  const [shakeKey, setShakeKey] = React.useState(0);
  const toastTimerRef = React.useRef<number | null>(null);

  // ─── initial load ─────────────────────────────────────────────────
  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const res = await fetch("/api/notifications?limit=" + NOTIF_LIST_LIMIT, {
          credentials: "include",
        });
        if (!res.ok) return;
        const data = (await res.json()) as { items?: NotifItem[]; unread_count?: number };
        if (cancelled) return;
        setItems(data.items ?? []);
        setUnread(data.unread_count ?? 0);
      } catch {
        /* offline-first; the WS subscription below will pick up new items */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // ─── ws subscription ──────────────────────────────────────────────
  React.useEffect(() => {
    const off = wsClient.subscribe("notifications", (evt) => {
      if (evt.type !== "notification") return;
      const payload = evt.payload as NotifItem | string | undefined;
      let n: NotifItem | null = null;
      if (typeof payload === "string") {
        try { n = JSON.parse(payload) as NotifItem; } catch { return; }
      } else if (payload && typeof payload === "object") {
        n = payload as NotifItem;
      }
      if (!n || !n.id) return;
      const item: NotifItem = { ...n, read: false };
      setItems((curr) => {
        // Prepend; cap to 200 to avoid runaway memory.
        const next = [item, ...curr.filter((x) => x.id !== item.id)];
        return next.slice(0, 200);
      });
      setUnread((u) => u + 1);
      setShakeKey((k) => k + 1);
      // Reset toast timer + show new one.
      if (toastTimerRef.current) {
        window.clearTimeout(toastTimerRef.current);
      }
      setToast(item);
      toastTimerRef.current = window.setTimeout(() => setToast(null), TOAST_TTL_MS);
    });
    return () => {
      off();
      if (toastTimerRef.current) {
        window.clearTimeout(toastTimerRef.current);
        toastTimerRef.current = null;
      }
    };
  }, []);

  const open = React.useCallback(() => setIsOpen(true), []);
  const close = React.useCallback(() => setIsOpen(false), []);
  const toggle = React.useCallback(() => setIsOpen((v) => !v), []);
  const dismissToast = React.useCallback(() => {
    if (toastTimerRef.current) {
      window.clearTimeout(toastTimerRef.current);
      toastTimerRef.current = null;
    }
    setToast(null);
  }, []);

  const markRead = React.useCallback(async (id: string) => {
    setItems((curr) => curr.map((n) => (n.id === id ? { ...n, read: true } : n)));
    setUnread((u) => Math.max(0, u - 1));
    try {
      await fetch(`/api/notifications/${encodeURIComponent(id)}/read`, {
        method: "POST",
        credentials: "include",
      });
    } catch {
      /* ignore — UI optimistic */
    }
  }, []);

  const markAllRead = React.useCallback(async () => {
    setItems((curr) => curr.map((n) => ({ ...n, read: true })));
    setUnread(0);
    try {
      await fetch("/api/notifications/read-all", {
        method: "POST",
        credentials: "include",
      });
    } catch {
      /* ignore */
    }
  }, []);

  const value: NotifCenterCtx = {
    items, unread, isOpen, toast,
    open, close, toggle,
    markRead, markAllRead, dismissToast,
    shakeKey,
  };

  return (
    <Ctx.Provider value={value}>
      {children}
      <NotifDrawer />
      <NotifToast />
    </Ctx.Provider>
  );
}

// ─── Bell button (mount in Topbar) ─────────────────────────────────

export function NotifBell() {
  const { unread, toggle, shakeKey } = useNotifCenter();
  const hasUnread = unread > 0;
  return (
    <button
      type="button"
      onClick={toggle}
      title={hasUnread ? `${unread} sin leer` : "Notificaciones"}
      aria-label={hasUnread ? `${unread} notificaciones sin leer` : "Notificaciones"}
      className="relative inline-flex items-center justify-center cursor-pointer"
      style={{
        width: 32, height: 32,
        border: "1px solid var(--color-line)",
        background: "rgba(12,18,40,0.45)",
        color: hasUnread ? "var(--color-orange)" : "var(--color-dim)",
        clipPath: "polygon(8px 0, 100% 0, calc(100% - 8px) 100%, 0 100%)",
      }}
    >
      <Bell
        size={14}
        // Re-mount the icon on every shakeKey bump to retrigger the animation.
        key={shakeKey}
        className={hasUnread ? "anim-bell-shake" : undefined}
      />
      {hasUnread && (
        <span
          className="anim-ring-pulse"
          style={{
            position: "absolute",
            top: -3, right: -3,
            minWidth: 16, height: 16,
            padding: "0 4px",
            background: "var(--color-orange)",
            color: "var(--color-bg)",
            fontFamily: "var(--font-display)",
            fontWeight: 700,
            fontSize: 9,
            display: "grid",
            placeItems: "center",
            borderRadius: 8,
          }}
        >
          {unread > 99 ? "99+" : unread}
        </span>
      )}
    </button>
  );
}

// ─── Drawer (right slide-in) ──────────────────────────────────────

function NotifDrawer() {
  const { items, isOpen, close, markRead, markAllRead, unread } = useNotifCenter();
  if (!isOpen) return null;
  return (
    <>
      {/* backdrop */}
      <div
        onClick={close}
        style={{
          position: "fixed", inset: 0, zIndex: 80,
          background: "rgba(6,8,20,0.55)",
          backdropFilter: "blur(2px)",
        }}
      />
      <aside
        className="anim-drawer-in"
        style={{
          position: "fixed",
          top: 0, right: 0, bottom: 0,
          width: 380,
          maxWidth: "92vw",
          zIndex: 81,
          background: "rgba(10,15,36,0.96)",
          borderLeft: "2px solid var(--color-orange)",
          boxShadow: "-12px 0 32px rgba(255,159,67,0.18), 0 0 32px rgba(255,78,214,0.08)",
          display: "flex", flexDirection: "column",
        }}
      >
        <div
          style={{
            padding: "14px 18px",
            borderBottom: "1px solid var(--color-line)",
            display: "flex", alignItems: "center", justifyContent: "space-between",
          }}
        >
          <div>
            <div className="font-display tracking-hud uppercase" style={{ color: "var(--color-orange)", fontSize: 13, fontWeight: 700 }}>
              Notificaciones
            </div>
            <div className="font-mono tracking-hud-tight" style={{ fontSize: 9, color: "var(--color-dim)", marginTop: 2 }}>
              {unread} sin leer · {items.length} totales
            </div>
          </div>
          <div className="flex items-center gap-2">
            {unread > 0 && (
              <button
                type="button"
                onClick={() => void markAllRead()}
                className="cursor-pointer clip-tag"
                style={{
                  padding: "4px 10px",
                  fontSize: 9,
                  letterSpacing: "0.18em",
                  textTransform: "uppercase",
                  color: "var(--color-lime)",
                  border: "1px solid var(--color-lime)",
                  background: "rgba(163,255,78,0.08)",
                }}
              >
                marcar leídas
              </button>
            )}
            <button
              type="button"
              onClick={close}
              className="cursor-pointer"
              aria-label="Cerrar"
              style={{
                width: 26, height: 26,
                border: "1px solid var(--color-line)",
                color: "var(--color-dim)",
                background: "rgba(255,255,255,0.02)",
                display: "grid", placeItems: "center",
              }}
            >
              <X size={13} />
            </button>
          </div>
        </div>
        <div style={{ flex: 1, overflowY: "auto", padding: 12 }}>
          {items.length === 0 && (
            <div style={{ padding: 24, textAlign: "center", color: "var(--color-dim)", fontSize: 11 }}>
              sin notificaciones
            </div>
          )}
          {items.map((n) => (
            <NotifRow key={n.id} n={n} onClick={() => !n.read && void markRead(n.id)} />
          ))}
        </div>
      </aside>
    </>
  );
}

function NotifRow({ n, onClick }: { n: NotifItem; onClick: () => void }) {
  const accent =
    n.severity === "error" ? "var(--color-danger)" :
    n.severity === "warn"  ? "var(--color-warn)"   :
                             "var(--color-cyan)";
  const ts = relTime(n.ts);
  return (
    <button
      type="button"
      onClick={onClick}
      className="w-full text-left mb-2 anim-fade-in-up cursor-pointer"
      style={{
        opacity: n.read ? 0.6 : 1,
        padding: "10px 12px",
        border: "1px solid var(--color-line)",
        background: n.read ? "rgba(255,255,255,0.02)" : "rgba(94,240,255,0.04)",
        clipPath: "polygon(10px 0, 100% 0, 100% calc(100% - 10px), calc(100% - 10px) 100%, 0 100%, 0 10px)",
        transition: "transform .15s",
      }}
    >
      <div className="flex items-center gap-2 mb-1" style={{ fontSize: 9, letterSpacing: "0.12em", textTransform: "uppercase" }}>
        <span
          className={n.read ? undefined : "anim-heartbeat"}
          style={{
            width: 6, height: 6, borderRadius: 999,
            background: accent,
            boxShadow: n.read ? undefined : `0 0 6px ${accent}`,
          }}
        />
        <span style={{ color: "var(--color-magenta)" }}>{n.kind}</span>
        <span style={{ color: "var(--color-dim)", marginLeft: "auto" }}>{ts}</span>
      </div>
      <div style={{ color: "var(--color-fg)", fontSize: 12, fontWeight: 600 }}>{n.title}</div>
      {n.body && (
        <div style={{ color: "var(--color-dim)", fontSize: 11, marginTop: 4, lineHeight: 1.4 }}>{n.body}</div>
      )}
    </button>
  );
}

// ─── Toast (top-right) ────────────────────────────────────────────

function NotifToast() {
  const { toast, open, dismissToast } = useNotifCenter();
  if (!toast) return null;
  const accent =
    toast.severity === "error" ? "var(--color-danger)" :
    toast.severity === "warn"  ? "var(--color-warn)"   :
                                 "var(--color-cyan)";
  return (
    <div
      className="anim-toast-pop"
      style={{
        position: "fixed",
        top: 16, right: 16,
        zIndex: 90,
        width: 320, maxWidth: "calc(100vw - 32px)",
        background: "rgba(10,15,36,0.96)",
        border: `1px solid ${accent}`,
        clipPath: "polygon(12px 0, 100% 0, 100% calc(100% - 12px), calc(100% - 12px) 100%, 0 100%, 0 12px)",
        overflow: "hidden",
      }}
    >
      <button
        type="button"
        onClick={() => { dismissToast(); open(); }}
        className="w-full text-left cursor-pointer"
        style={{ padding: "12px 14px" }}
        title="Click para abrir el drawer"
      >
        <div className="flex items-center gap-2" style={{ fontSize: 9, letterSpacing: "0.12em", textTransform: "uppercase" }}>
          <Bell size={12} style={{ color: accent }} />
          <span style={{ color: "var(--color-magenta)" }}>{toast.kind}</span>
          <span style={{ color: "var(--color-dim)", marginLeft: "auto" }}>{relTime(toast.ts)}</span>
        </div>
        <div style={{ color: "var(--color-fg)", fontSize: 12, fontWeight: 600, marginTop: 4 }}>{toast.title}</div>
        {toast.body && (
          <div style={{ color: "var(--color-dim)", fontSize: 11, marginTop: 4, lineHeight: 1.4 }}>
            {toast.body.length > 120 ? toast.body.slice(0, 120) + "…" : toast.body}
          </div>
        )}
      </button>
      {/* Progress bar — pure CSS, drains over TOAST_TTL_MS via inline animation. */}
      <div style={{ height: 2, background: "rgba(120,255,220,0.08)", overflow: "hidden" }}>
        <div
          style={{
            height: "100%",
            background: `linear-gradient(90deg, var(--color-cyan), var(--color-magenta))`,
            transformOrigin: "left",
            animation: `notif-progress ${TOAST_TTL_MS}ms linear forwards`,
          }}
        />
      </div>
      <style>{`@keyframes notif-progress { from { transform: scaleX(1); } to { transform: scaleX(0); } }`}</style>
    </div>
  );
}

// ─── helpers ─────────────────────────────────────────────────────

function relTime(unix: number): string {
  if (!unix) return "—";
  const diff = Math.floor(Date.now() / 1000) - unix;
  if (diff < 0) return "ahora";
  if (diff < 60) return `${diff}s`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h`;
  return `${Math.floor(diff / 86400)}d`;
}
