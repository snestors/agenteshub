import * as React from "react";
import {
  MessageSquare,
  FolderKanban,
  Activity,
  LogOut,
  Bell,
  Lock,
  Network,
  Sparkles,
  Tag,
  X,
} from "lucide-react";
import { useLocation, useNavigate } from "react-router-dom";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";
import { useNotifications } from "@/lib/notifications";
import { MOBILE_NAV_OPEN_EVENT } from "@/lib/mobileNav";
import { SidebarStats } from "@/components/SidebarStats";

interface NavItem {
  to: string;
  label: string;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number }>;
  /** kind prefix used to count unread notifications for this section, if any */
  notifPrefix?: string;
  accent: "cyan" | "magenta" | "lime" | "orange";
}

const ITEMS: NavItem[] = [
  { to: "/", label: "Chat", icon: MessageSquare, accent: "cyan" },
  { to: "/projects", label: "Proyectos", icon: FolderKanban, accent: "lime", notifPrefix: "project_turn" },
  { to: "/diagrams", label: "Diagramas", icon: Network, accent: "cyan" },
  { to: "/system", label: "Sistema", icon: Activity, accent: "lime" },
  { to: "/vault", label: "Vault", icon: Lock, accent: "orange" },
  { to: "/skills", label: "Skills", icon: Sparkles, accent: "cyan" },
  { to: "/releases", label: "Releases", icon: Tag, accent: "lime" },
];

const ACCENT_VAR: Record<NavItem["accent"], string> = {
  cyan: "var(--color-cyan)",
  magenta: "var(--color-magenta)",
  lime: "var(--color-lime)",
  orange: "var(--color-orange)",
};

export function Sidebar({ username }: { username?: string }) {
  const { unreadByKindPrefix, unreadCount, openDrawer } = useNotifications();
  const location = useLocation();
  const navigate = useNavigate();

  async function handleLogout() {
    try {
      await api.logout();
    } catch {
      // ignore — cookie may already be invalid
    }
    window.location.href = "/login";
  }

  function handleNavClick(item: NavItem) {
    if (item.notifPrefix && unreadByKindPrefix(item.notifPrefix) > 0) {
      openDrawer();
      return;
    }
    navigate(item.to);
  }

  return (
    <aside className="hidden md:flex w-[220px] shrink-0 h-full flex-col border-r border-[var(--color-line)] bg-[rgba(10,15,36,0.55)] backdrop-blur-sm relative z-10">
      {/* brand */}
      <div className="px-4 py-4 border-b border-[var(--color-line)]">
        <div className="flex items-center gap-3">
          <div
            className="w-9 h-9 relative flex items-center justify-center"
            style={{
              clipPath:
                "polygon(30% 0, 100% 0, 100% 70%, 70% 100%, 0 100%, 0 30%)",
              background:
                "linear-gradient(135deg, var(--color-magenta), var(--color-cyan))",
            }}
          >
            <div
              className="absolute inset-[2px] flex items-center justify-center"
              style={{
                clipPath:
                  "polygon(30% 0, 100% 0, 100% 70%, 70% 100%, 0 100%, 0 30%)",
                background: "var(--color-bg)",
                color: "var(--color-lime)",
                fontFamily: "var(--font-display)",
                fontWeight: 800,
                fontSize: 14,
              }}
            >
              ◆
            </div>
          </div>
          <div className="leading-none">
            <div className="font-display font-bold text-[14px] tracking-hud text-[var(--color-fg)]">
              AGENT
              <span className="text-[var(--color-magenta)]">//</span>
              HUB
            </div>
            <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud mt-1">
              v0 · NODE-42
            </div>
          </div>
        </div>
      </div>

      {/* nav */}
      <nav className="flex-1 px-3 py-3 flex flex-col gap-1 overflow-y-auto">
        {ITEMS.map((item) => {
          const Icon = item.icon;
          const accentColor = ACCENT_VAR[item.accent];
          const badge = item.notifPrefix
            ? unreadByKindPrefix(item.notifPrefix)
            : 0;
          const isActive =
            item.to === "/"
              ? location.pathname === "/"
              : location.pathname.startsWith(item.to);
          return (
            <button
              key={item.to}
              type="button"
              onClick={() => handleNavClick(item)}
              className={cn(
                "group relative flex items-center gap-3 px-3 py-2 font-mono text-[11px] uppercase tracking-hud-tight transition-colors clip-tag cursor-pointer text-left",
                isActive
                  ? "bg-[rgba(94,240,255,0.08)] text-[var(--color-fg)]"
                  : "text-[var(--color-dim)] hover:text-[var(--color-fg)] hover:bg-[rgba(120,255,220,0.04)]",
              )}
              style={
                isActive
                  ? { borderLeft: `2px solid ${accentColor}` }
                  : { borderLeft: "2px solid transparent" }
              }
            >
              <Icon size={14} strokeWidth={1.6} />
              <span className="flex-1">{item.label}</span>
              {badge > 0 && (
                <span
                  className="font-display font-bold text-[10px] px-1.5"
                  style={{
                    color: accentColor,
                    background: `${accentColor}15`,
                  }}
                >
                  {badge > 99 ? "99+" : badge}
                </span>
              )}
              {isActive && (
                <span
                  className="absolute right-2 w-1 h-1 rounded-full"
                  style={{
                    background: accentColor,
                    boxShadow: `0 0 6px ${accentColor}`,
                  }}
                />
              )}
            </button>
          );
        })}
      </nav>

      {/* live system stats */}
      <SidebarStats />

      {/* notifications button */}
      <div className="px-3 pt-2">
        <button
          type="button"
          onClick={openDrawer}
          className="relative w-full flex items-center gap-2 px-3 py-1.5 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
          style={{ borderColor: "var(--color-line)" }}
          title="Notificaciones"
        >
          <Bell size={12} strokeWidth={1.6} />
          <span className="flex-1 text-left">notificaciones</span>
          {unreadCount > 0 && (
            <span
              className="font-display font-bold text-[10px] px-1.5"
              style={{
                color: "var(--color-orange)",
                background: "rgba(255,184,108,0.15)",
              }}
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
        </button>
      </div>

      {/* user footer */}
      <div className="px-3 py-3 border-t border-[var(--color-line)]">
        <div className="flex items-center gap-2 px-2 py-2 clip-tag bg-[rgba(94,240,255,0.04)]">
          <div
            className="w-6 h-6 flex items-center justify-center font-display font-bold text-[10px]"
            style={{
              background:
                "linear-gradient(135deg, var(--color-cyan), var(--color-magenta))",
              color: "var(--color-bg)",
            }}
          >
            {(username ?? "?").slice(0, 2).toUpperCase()}
          </div>
          <div className="flex-1 min-w-0">
            <div className="font-mono text-[11px] text-[var(--color-fg)] truncate">
              {username ?? "—"}
            </div>
            <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight">
              ONLINE
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="text-[var(--color-dim)] hover:text-[var(--color-danger)] transition-colors p-1 cursor-pointer"
            title="Cerrar sesión"
          >
            <LogOut size={13} strokeWidth={1.6} />
          </button>
        </div>
      </div>
    </aside>
  );
}

export function MobileNav({ username }: { username?: string }) {
  const { unreadByKindPrefix, unreadCount, openDrawer } = useNotifications();
  const location = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = React.useState(false);

  React.useEffect(() => {
    function onOpen() {
      setOpen(true);
    }
    window.addEventListener(MOBILE_NAV_OPEN_EVENT, onOpen);
    return () => window.removeEventListener(MOBILE_NAV_OPEN_EVENT, onOpen);
  }, []);

  React.useEffect(() => {
    setOpen(false);
  }, [location.pathname]);

  React.useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  async function handleLogout() {
    try {
      await api.logout();
    } catch {
      // ignore — cookie may already be invalid
    }
    window.location.href = "/login";
  }

  function handleNavClick(item: NavItem) {
    if (item.notifPrefix && unreadByKindPrefix(item.notifPrefix) > 0) {
      openDrawer();
      setOpen(false);
      return;
    }
    navigate(item.to);
    setOpen(false);
  }

  return (
    <>
      {open && (
        <div
          className="fixed inset-0 z-50 md:hidden"
          role="dialog"
          aria-modal="true"
          aria-label="Navegación mobile"
        >
          <div
            className="absolute inset-0 bg-black/65 backdrop-blur-[1px]"
            onClick={() => setOpen(false)}
            aria-hidden="true"
          />
          <aside
            className="absolute inset-y-0 left-0 flex w-[280px] max-w-[86vw] flex-col border-r border-[var(--color-line)] bg-[rgba(10,15,36,0.97)] shadow-2xl"
            style={{
              paddingTop: "env(safe-area-inset-top)",
              paddingBottom: "env(safe-area-inset-bottom)",
              boxShadow: "0 0 28px rgba(94,240,255,0.25)",
            }}
          >
            <div className="flex items-center justify-between gap-3 border-b border-[var(--color-line)] px-4 py-4">
              <div className="leading-none">
                <div className="font-display text-[14px] font-bold tracking-hud text-[var(--color-fg)]">
                  AGENT<span className="text-[var(--color-magenta)]">//</span>HUB
                </div>
                <div className="mt-1 font-mono text-[9px] tracking-hud text-[var(--color-dim)]">
                  mobile nav
                </div>
              </div>
              <button
                type="button"
                onClick={() => setOpen(false)}
                className="p-2 clip-tag text-[var(--color-dim)] hover:text-[var(--color-fg)]"
                style={{ border: "1px solid var(--color-line)" }}
                aria-label="Cerrar navegación"
              >
                <X size={15} strokeWidth={1.8} />
              </button>
            </div>

            <nav className="flex-1 overflow-y-auto px-3 py-3">
              {ITEMS.map((item) => {
                const Icon = item.icon;
                const accentColor = ACCENT_VAR[item.accent];
                const badge = item.notifPrefix
                  ? unreadByKindPrefix(item.notifPrefix)
                  : 0;
                const isActive =
                  item.to === "/"
                    ? location.pathname === "/"
                    : location.pathname.startsWith(item.to);
                return (
                  <button
                    key={item.to}
                    type="button"
                    onClick={() => handleNavClick(item)}
                    className={cn(
                      "relative mb-1 flex w-full items-center gap-3 px-3 py-2.5 clip-tag font-mono text-[11px] uppercase tracking-hud-tight transition-colors",
                      isActive
                        ? "text-[var(--color-fg)]"
                        : "text-[var(--color-dim)]",
                    )}
                    style={{
                      borderLeft: `2px solid ${isActive ? accentColor : "transparent"}`,
                      background: isActive ? `${accentColor}14` : "rgba(255,255,255,0.03)",
                    }}
                  >
                    <Icon size={15} strokeWidth={1.7} />
                    <span className="flex-1 text-left">{item.label}</span>
                    {badge > 0 && (
                      <span
                        className="px-1.5 font-display text-[10px] font-bold"
                        style={{ color: accentColor, background: `${accentColor}15` }}
                      >
                        {badge > 99 ? "99+" : badge}
                      </span>
                    )}
                  </button>
                );
              })}
            </nav>

            <div className="border-t border-[var(--color-line)] px-3 py-3">
              <button
                type="button"
                onClick={() => {
                  openDrawer();
                  setOpen(false);
                }}
                className="mb-2 flex w-full items-center gap-2 px-3 py-2 clip-tag font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]"
                style={{ border: "1px solid var(--color-line)", background: "rgba(255,255,255,0.03)" }}
              >
                <Bell size={13} strokeWidth={1.7} />
                <span className="flex-1 text-left">notificaciones</span>
          {unreadCount > 0 && (
            <span
                    className="px-1.5 font-display text-[10px] font-bold"
              style={{
                color: "var(--color-orange)",
                      background: "rgba(255,184,108,0.15)",
              }}
            >
              {unreadCount > 99 ? "99+" : unreadCount}
            </span>
          )}
              </button>

        <button
          type="button"
          onClick={handleLogout}
                className="flex w-full items-center gap-2 px-3 py-2 clip-tag font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)]"
          style={{ border: "1px solid var(--color-line)", background: "rgba(255,255,255,0.03)" }}
          aria-label={`Cerrar sesión${username ? ` (${username})` : ""}`}
        >
                <LogOut size={13} strokeWidth={1.7} />
                <span className="flex-1 text-left">{username ?? "sesión"}</span>
                <span>salir</span>
        </button>
      </div>
          </aside>
        </div>
      )}
    </>
  );
}
