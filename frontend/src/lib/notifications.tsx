import * as React from "react";
import { useLocation, useNavigate } from "react-router-dom";
import { Bot, AlertTriangle, Info, X } from "lucide-react";
import { wsClient } from "@/lib/wsClient";
import { api } from "@/lib/api";
import {
  getFirebasePushSupport,
  registerFirebasePush,
  registerFirebasePushIfGranted,
  type FirebasePushResult,
} from "@/lib/firebasePush";

// Web Audio API blip — synthesizes a short two-tone notification on demand.
// We don't ship an audio file because (a) one less network request, (b) no
// licensing question, and (c) it's a couple of lines.
let audioCtx: AudioContext | null = null;
function playBlip(severity: "info" | "warn" | "error") {
  try {
    if (!audioCtx) {
      const Ctx = window.AudioContext || (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
      if (!Ctx) return;
      audioCtx = new Ctx();
    }
    const ctx = audioCtx;
    if (ctx.state === "suspended") void ctx.resume();
    const now = ctx.currentTime;
    const freq = severity === "error" ? 320 : severity === "warn" ? 520 : 740;
    const tones = severity === "error" ? [freq, freq * 0.7] : [freq, freq * 1.5];
    tones.forEach((f, i) => {
      const osc = ctx.createOscillator();
      const gain = ctx.createGain();
      osc.type = "sine";
      osc.frequency.value = f;
      const start = now + i * 0.12;
      gain.gain.setValueAtTime(0, start);
      gain.gain.linearRampToValueAtTime(0.18, start + 0.01);
      gain.gain.exponentialRampToValueAtTime(0.001, start + 0.18);
      osc.connect(gain).connect(ctx.destination);
      osc.start(start);
      osc.stop(start + 0.2);
    });
  } catch {
    // ignore — autoplay policy or unavailable
  }
}

function contextNumber(ctx: Record<string, unknown> | undefined, key: string): number | null {
  const value = ctx?.[key];
  if (typeof value === "number" && Number.isFinite(value) && value > 0) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    if (Number.isFinite(parsed) && parsed > 0) return parsed;
  }
  return null;
}

// Map a notification to the exact route we should offer to navigate to.
function routeForNotification(n: Notification): string | null {
  if (n.kind.startsWith("main_turn")) return "/";
  if (n.kind.startsWith("agent_run")) {
    const agentID = contextNumber(n.context, "agent_id");
    return agentID ? `/agents/${agentID}` : "/agents";
  }
  if (n.kind.startsWith("project_turn")) {
    const projectID = contextNumber(n.context, "project_id");
    const sessionID = contextNumber(n.context, "session_id");
    if (projectID && sessionID) return `/projects/${projectID}/sessions/${sessionID}`;
    if (projectID) return `/projects/${projectID}`;
    return "/projects";
  }
  return null;
}

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
  unreadCount: number;
  unreadByKindPrefix: (prefix: string) => number;
  markAllRead: () => void;
  markRead: (id: string) => void;
  dismiss: (id: string) => void;
  clearRead: () => void;
  push: (n: Notification) => void;
  /** open the drawer programmatically (e.g. from the sidebar bell). */
  openDrawer: () => void;
  isDrawerOpen: boolean;
  closeDrawer: () => void;
}

const Ctx = React.createContext<NotificationsState | null>(null);

const MAX_KEEP = 50;
const TOAST_TTL_MS = 15000;

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

type PushUiStatus =
  | "idle"
  | "asking"
  | "registered"
  | "unsupported"
  | "denied"
  | "dismissed"
  | "error";

type PushState = {
  status: PushUiStatus;
  detail?: string;
};

function stateFromPushResult(res: FirebasePushResult): PushState {
  switch (res) {
    case "registered":
      return { status: "registered", detail: "push activo" };
    case "unsupported":
      return { status: "unsupported", detail: "Push no está disponible en este navegador/contexto. Probá desde HTTPS o la PWA instalada." };
    case "denied":
      return { status: "denied", detail: "El navegador tiene bloqueadas las notificaciones para este sitio." };
    case "dismissed":
      return { status: "dismissed", detail: "El navegador no mostró el permiso todavía; lo intento de nuevo con tu próximo toque." };
    case "no-token":
      return { status: "error", detail: "Firebase no devolvió token. Tocá para reintentar." };
  }
}

function pushButtonLabel(status: PushUiStatus): string {
  switch (status) {
    case "asking":
      return "pidiendo permiso";
    case "registered":
      return "push activo";
    case "unsupported":
      return "push no disponible";
    case "denied":
      return "push bloqueado";
    case "dismissed":
      return "permitir push";
    case "error":
      return "reintentar push";
    default:
      return "activar push";
  }
}

export function NotificationProvider({ children }: { children: React.ReactNode }) {
  const [items, setItems] = React.useState<Notification[]>([]);
  const [isDrawerOpen, setDrawerOpen] = React.useState(false);
  const [pushState, setPushState] = React.useState<PushState>({ status: "idle" });

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

  const markRead = React.useCallback((id: string) => {
    setItems((curr) => curr.map((i) => (i.id === id ? { ...i, read: true } : i)));
  }, []);

  const clearRead = React.useCallback(() => {
    setItems((curr) => curr.filter((i) => !i.read));
  }, []);

  const openDrawer = React.useCallback(() => setDrawerOpen(true), []);
  const closeDrawer = React.useCallback(() => setDrawerOpen(false), []);

  const unreadCount = React.useMemo(
    () => items.reduce((acc, i) => (!i.read ? acc + 1 : acc), 0),
    [items]
  );

  const unreadByKindPrefix = React.useCallback(
    (prefix: string) =>
      items.reduce((acc, i) => (!i.read && i.kind.startsWith(prefix) ? acc + 1 : acc), 0),
    [items]
  );

  const enablePush = React.useCallback(async () => {
    try {
      setPushState({ status: "asking", detail: "pidiendo permiso al navegador…" });
      const res = await registerFirebasePush();
      setPushState(stateFromPushResult(res));
    } catch {
      setPushState({ status: "error", detail: "No pude registrar el token FCM. Tocá para reintentar." });
    }
  }, []);

  React.useEffect(() => {
    let cancelled = false;
    const retryOnGesture = () => {
      window.removeEventListener("pointerdown", retryOnGesture, true);
      window.removeEventListener("keydown", retryOnGesture, true);
      void enablePush();
    };

    void (async () => {
      const support = await getFirebasePushSupport();
      if (cancelled) return;
      if (!support.ok) {
        setPushState({ status: "unsupported", detail: support.message });
        return;
      }

      if (Notification.permission === "granted") {
        const ok = await registerFirebasePushIfGranted();
        if (!cancelled && ok) setPushState({ status: "registered", detail: "push activo" });
        return;
      }
      if (Notification.permission === "denied") {
        setPushState({ status: "denied", detail: "El navegador tiene bloqueadas las notificaciones para este sitio." });
        return;
      }

      // Intento automático: en Chrome mobile suele mostrar el permiso directo.
      // Si el navegador exige gesto de usuario, armamos un retry invisible con
      // el próximo tap/tecla para que Nestor no tenga que buscar el botón.
      setPushState({ status: "asking", detail: "pidiendo permiso al navegador…" });
      const res = await registerFirebasePush();
      if (cancelled) return;
      setPushState(stateFromPushResult(res));
      if (res === "dismissed") {
        window.addEventListener("pointerdown", retryOnGesture, { capture: true, once: true, passive: true });
        window.addEventListener("keydown", retryOnGesture, { capture: true, once: true });
      }
    })();

    return () => {
      cancelled = true;
      window.removeEventListener("pointerdown", retryOnGesture, true);
      window.removeEventListener("keydown", retryOnGesture, true);
    };
  }, [enablePush]);

  // Subscribe globally to the 'notifications' topic.
  React.useEffect(() => {
    const off = wsClient.subscribe("notifications", (env) => {
      if (env.type !== "notification") return;
      const n = parsePayload(env.payload);
      if (!n || !n.id) return;
      push(n);
      playBlip(n.severity ?? "info");
    });
    return off;
  }, [push]);

  const value = React.useMemo<NotificationsState>(
    () => ({
      items,
      unreadCount,
      unreadByKindPrefix,
      markAllRead,
      markRead,
      dismiss,
      clearRead,
      push,
      openDrawer,
      isDrawerOpen,
      closeDrawer,
    }),
    [
      items,
      unreadCount,
      unreadByKindPrefix,
      markAllRead,
      markRead,
      dismiss,
      clearRead,
      push,
      openDrawer,
      isDrawerOpen,
      closeDrawer,
    ]
  );

  return (
    <Ctx.Provider value={value}>
      {children}
      <RoutedToastStack items={items} dismiss={dismiss} markRead={markRead} />
      {isDrawerOpen && (
        <NotificationDrawer
          items={items}
          pushState={pushState}
          onEnablePush={() => void enablePush()}
          onClose={closeDrawer}
          onDismiss={dismiss}
          onMarkRead={markRead}
          onMarkAllRead={markAllRead}
          onClearRead={clearRead}
        />
      )}
    </Ctx.Provider>
  );
}

// ─── Confirm modal ───────────────────────────────────────────

interface ConfirmModalProps {
  notif: Notification;
  route: string;
  onConfirm: () => void;
  onCancel: () => void;
}

// ─── Notification drawer (history of unread + recently read) ────

interface DrawerProps {
  items: Notification[];
  onClose: () => void;
  onDismiss: (id: string) => void;
  onMarkRead: (id: string) => void;
  onMarkAllRead: () => void;
  onClearRead: () => void;
  pushState: PushState;
  onEnablePush: () => void;
}

function NotificationDrawer({
  items,
  onClose,
  onDismiss,
  onMarkRead,
  onMarkAllRead,
  onClearRead,
  pushState,
  onEnablePush,
}: DrawerProps) {
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const [confirming, setConfirming] = React.useState<{ notif: Notification; route: string } | null>(
    null
  );

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const unread = items.filter((i) => !i.read);
  const read = items.filter((i) => i.read);

  function handleEntryClick(n: Notification) {
    const route = routeForNotification(n);
    if (!route || pathname === route || pathname.startsWith(route + "/")) {
      onMarkRead(n.id);
      return;
    }
    setConfirming({ notif: n, route });
  }

  return (
    <>
      <div
        className="fixed inset-0 z-[55]"
        style={{ background: "rgba(2, 4, 14, 0.55)", backdropFilter: "blur(2px)" }}
        onClick={onClose}
      >
        <div
          className="absolute right-0 top-0 h-full w-[380px] max-w-[92vw] flex flex-col border-l"
          style={{
            background: "rgba(10, 15, 36, 0.97)",
            borderColor: "var(--color-line)",
            boxShadow: "-8px 0 24px rgba(0,0,0,0.5)",
          }}
          onClick={(e) => e.stopPropagation()}
        >
          {/* header */}
          <div className="px-4 py-3 flex items-center justify-between border-b" style={{ borderColor: "var(--color-line)" }}>
            <div>
              <div className="font-display font-semibold text-[12px] uppercase tracking-hud" style={{ color: "var(--color-cyan)" }}>
                ◂ notificaciones
              </div>
              <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight uppercase mt-0.5">
                {unread.length} sin leer · {read.length} leídas
              </div>
            </div>
            <button
              type="button"
              onClick={onClose}
              className="text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer p-1 transition-colors"
              aria-label="cerrar"
            >
              <X size={14} strokeWidth={1.8} />
            </button>
          </div>

          {/* actions */}
          <div className="px-4 py-2 flex flex-wrap gap-2 border-b" style={{ borderColor: "var(--color-line)" }}>
            <button
              type="button"
              onClick={onMarkAllRead}
              disabled={unread.length === 0}
              className="px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)" }}
            >
              marcar todas
            </button>
            <button
              type="button"
              onClick={onClearRead}
              disabled={read.length === 0}
              className="px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)" }}
            >
              limpiar leídas
            </button>
            <button
              type="button"
              onClick={onEnablePush}
              disabled={pushState.status === "registered" || pushState.status === "asking"}
              className="px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer transition-colors"
              style={{ borderColor: pushState.status === "registered" ? "var(--color-lime)" : "var(--color-line)", color: pushState.status === "registered" ? "var(--color-lime)" : undefined }}
              title={pushState.detail || "Activar push FCM"}
            >
              {pushButtonLabel(pushState.status)}
            </button>
            {pushState.detail && pushState.status !== "registered" && (
              <div className="basis-full font-mono text-[9px] text-[var(--color-dim)] leading-snug">
                {pushState.detail}
              </div>
            )}
          </div>

          {/* list */}
          <div className="flex-1 overflow-y-auto px-2 py-2 space-y-1.5">
            {items.length === 0 && (
              <div className="px-2 py-8 text-center font-mono text-[11px] text-[var(--color-dim)] italic">
                sin notificaciones
              </div>
            )}
            {unread.map((n) => (
              <DrawerEntry
                key={n.id}
                n={n}
                emphasis
                onClick={() => handleEntryClick(n)}
                onDismiss={() => onDismiss(n.id)}
              />
            ))}
            {unread.length > 0 && read.length > 0 && (
              <div className="my-2 mx-2 h-px" style={{ background: "var(--color-line)" }} />
            )}
            {read.map((n) => (
              <DrawerEntry
                key={n.id}
                n={n}
                emphasis={false}
                onClick={() => handleEntryClick(n)}
                onDismiss={() => onDismiss(n.id)}
              />
            ))}
          </div>
        </div>
      </div>

      {confirming && (
        <ConfirmModal
          notif={confirming.notif}
          route={confirming.route}
          onConfirm={() => {
            navigate(confirming.route);
            onMarkRead(confirming.notif.id);
            setConfirming(null);
            onClose();
          }}
          onCancel={() => {
            setConfirming(null);
          }}
        />
      )}
    </>
  );
}

function DrawerEntry({
  n,
  emphasis,
  onClick,
  onDismiss,
}: {
  n: Notification;
  emphasis: boolean;
  onClick: () => void;
  onDismiss: () => void;
}) {
  const accent =
    n.severity === "error"
      ? "var(--color-danger)"
      : n.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  const Icon = n.severity === "error" ? AlertTriangle : n.kind.startsWith("agent_run") ? Bot : Info;
  const elapsed = Math.max(0, Math.floor((Date.now() / 1000 - n.ts)));
  const elapsedStr =
    elapsed < 60 ? `${elapsed}s` : elapsed < 3600 ? `${Math.floor(elapsed / 60)}m` : `${Math.floor(elapsed / 3600)}h`;

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={onClick}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick();
        }
      }}
      className={`flex gap-2 px-2 py-2 clip-tag border cursor-pointer transition-colors ${
        emphasis ? "" : "opacity-60"
      } hover:bg-[rgba(94,240,255,0.04)]`}
      style={{
        borderColor: emphasis ? accent + "60" : "var(--color-line)",
        background: emphasis ? `${accent}08` : "transparent",
      }}
    >
      <Icon size={13} strokeWidth={1.6} style={{ color: accent, marginTop: 2 }} />
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline justify-between gap-2">
          <div
            className="font-mono text-[10px] uppercase tracking-hud-tight truncate"
            style={{ color: accent }}
          >
            {n.title}
          </div>
          <div className="font-mono text-[9px] text-[var(--color-dim)] shrink-0">{elapsedStr}</div>
        </div>
        {n.body && (
          <div className="font-mono text-[11px] text-[var(--color-fg)] mt-0.5 break-words line-clamp-2">
            {n.body}
          </div>
        )}
      </div>
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation();
          onDismiss();
        }}
        className="text-[var(--color-dim)] hover:text-[var(--color-danger)] cursor-pointer p-0.5 self-start transition-colors"
        aria-label="descartar"
      >
        <X size={11} strokeWidth={1.8} />
      </button>
    </div>
  );
}

function ConfirmModal({ notif, route, onConfirm, onCancel }: ConfirmModalProps) {
  const accent =
    notif.severity === "error"
      ? "var(--color-danger)"
      : notif.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";

  // Esc closes
  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
      if (e.key === "Enter") onConfirm();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onConfirm, onCancel]);

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center"
      style={{ background: "rgba(2, 4, 14, 0.65)", backdropFilter: "blur(2px)" }}
      onClick={onCancel}
    >
      <div
        className="clip-hud-sm border max-w-md w-[90%] mx-4"
        style={{
          background: "rgba(10, 15, 36, 0.97)",
          borderColor: accent,
          boxShadow: `0 0 24px ${accent}50`,
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="px-4 py-3 border-b" style={{ borderColor: accent + "30" }}>
          <div className="font-display font-semibold text-[11px] uppercase tracking-hud" style={{ color: accent }}>
            ◂ {notif.title}
          </div>
          <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight mt-1 uppercase">
            navegar · {route}
          </div>
        </div>
        <div className="px-4 py-3">
          {notif.body && (
            <div className="font-mono text-[12px] text-[var(--color-fg)] leading-[1.55] break-words mb-3">
              {notif.body}
            </div>
          )}
          <div className="font-mono text-[11px] text-[var(--color-dim)] mb-4">
            ¿Querés ir a <span style={{ color: accent }}>{route}</span> ahora?
          </div>
          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onCancel}
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)", background: "rgba(255,255,255,0.02)" }}
            >
              cancelar
            </button>
            <button
              type="button"
              onClick={onConfirm}
              autoFocus
              className="px-3 py-1.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight font-semibold cursor-pointer transition-all hover:scale-[1.02]"
              style={{
                color: "var(--color-bg)",
                background: accent,
                boxShadow: `0 0 8px ${accent}80`,
              }}
            >
              ir ahora ▸
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

// shouldToast hides the toast when the user is already on the screen that
// would naturally surface that event. The notification is still kept in the
// store so the badge counter is honest.
function shouldToast(n: Notification, pathname: string): boolean {
  const route = routeForNotification(n);
  if (!route) return true;
  return !(pathname === route || pathname.startsWith(route + "/"));
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
  markRead,
}: {
  items: Notification[];
  dismiss: (id: string) => void;
  markRead: (id: string) => void;
}) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const [now, setNow] = React.useState(Date.now());
  const [confirming, setConfirming] = React.useState<{ notif: Notification; route: string } | null>(null);

  React.useEffect(() => {
    const t = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(t);
  }, []);
  const visible = items
    .filter((i) => !i.read && now - i.ts * 1000 < TOAST_TTL_MS && shouldToast(i, pathname))
    .slice(0, 4);

  const handleClick = (n: Notification) => {
    const route = routeForNotification(n);
    if (!route || pathname === route || pathname.startsWith(route + "/")) {
      dismiss(n.id);
      return;
    }
    setConfirming({ notif: n, route });
  };

  return (
    <>
      {visible.length > 0 && (
        <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm pointer-events-none">
          {visible.map((n) => (
            <Toast key={n.id} n={n} onClose={() => dismiss(n.id)} onClick={() => handleClick(n)} />
          ))}
        </div>
      )}
      {confirming && (
        <ConfirmModal
          notif={confirming.notif}
          route={confirming.route}
          onConfirm={() => {
            navigate(confirming.route);
            markRead(confirming.notif.id);
            setConfirming(null);
          }}
          onCancel={() => {
            markRead(confirming.notif.id);
            setConfirming(null);
          }}
        />
      )}
    </>
  );
}

function Toast({
  n,
  onClose,
  onClick,
}: {
  n: Notification;
  onClose: () => void;
  onClick: () => void;
}) {
  const accent =
    n.severity === "error"
      ? "var(--color-danger)"
      : n.severity === "warn"
      ? "var(--color-orange)"
      : "var(--color-cyan)";
  const Icon = n.severity === "error" ? AlertTriangle : n.kind.startsWith("agent_run") ? Bot : Info;
  const isLongRunning = n.kind === "long_running_turn";
  const hasRoute = !isLongRunning && routeForNotification(n) !== null;

  const cancelScope = isLongRunning ? toStr(n.context?.scope) : "";
  const cancelID = isLongRunning ? toStr(n.context?.id) : "";
  const [cancelling, setCancelling] = React.useState(false);
  const [cancelMsg, setCancelMsg] = React.useState<string | null>(null);

  async function handleCancelRun() {
    if (!cancelScope || !cancelID || cancelling) return;
    setCancelling(true);
    try {
      await api.cancelRun(cancelScope, cancelID);
      setCancelMsg("Cancelado.");
      window.setTimeout(onClose, 800);
    } catch (err) {
      setCancelMsg(err instanceof Error ? err.message : "no se pudo cancelar");
      setCancelling(false);
    }
  }

  return (
    <div
      role={hasRoute ? "button" : undefined}
      tabIndex={hasRoute ? 0 : -1}
      onClick={hasRoute ? onClick : undefined}
      onKeyDown={
        hasRoute
          ? (e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onClick();
              }
            }
          : undefined
      }
      className={`pointer-events-auto clip-tag flex gap-3 px-3 py-2 backdrop-blur-sm border ${
        hasRoute ? "cursor-pointer hover:scale-[1.01] transition-transform" : ""
      }`}
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
        {isLongRunning && cancelScope && cancelID && (
          <div className="mt-2 flex items-center gap-2">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                void handleCancelRun();
              }}
              disabled={cancelling}
              className="px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
              style={{
                color: "var(--color-danger)",
                borderColor: "var(--color-danger)",
                background: "rgba(255, 71, 87, 0.06)",
              }}
            >
              {cancelling ? "cancelando…" : "cancelar"}
            </button>
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onClose();
              }}
              className="px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
              style={{ borderColor: "var(--color-line)", background: "rgba(255,255,255,0.02)" }}
            >
              continuar
            </button>
            {cancelMsg && (
              <span className="font-mono text-[10px] text-[var(--color-dim)]">
                {cancelMsg}
              </span>
            )}
          </div>
        )}
      </div>
      <button
        onClick={(e) => {
          e.stopPropagation();
          onClose();
        }}
        className="text-[var(--color-dim)] hover:text-[var(--color-fg)] transition-colors p-0.5 cursor-pointer self-start"
        aria-label="cerrar"
      >
        <X size={12} strokeWidth={1.8} />
      </button>
    </div>
  );
}

function toStr(v: unknown): string {
  if (typeof v === "string") return v;
  if (typeof v === "number" && Number.isFinite(v)) return String(v);
  return "";
}
