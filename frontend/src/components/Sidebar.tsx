import { NavLink } from "react-router-dom";
import {
  MessageSquare,
  FolderKanban,
  Bot,
  Hash,
  GitBranch,
  Activity,
  LogOut,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { api } from "@/lib/api";

interface NavItem {
  to: string;
  label: string;
  icon: React.ComponentType<{ size?: number; strokeWidth?: number }>;
  badge?: number;
  accent: "cyan" | "magenta" | "lime" | "orange";
}

const ITEMS: NavItem[] = [
  { to: "/", label: "Chat", icon: MessageSquare, badge: 0, accent: "cyan" },
  { to: "/projects", label: "Proyectos", icon: FolderKanban, badge: 0, accent: "lime" },
  { to: "/agents", label: "Mini-agentes", icon: Bot, badge: 0, accent: "orange" },
  { to: "/topics", label: "Topics", icon: Hash, badge: 0, accent: "magenta" },
  { to: "/subagents", label: "Sub-agentes", icon: GitBranch, badge: 0, accent: "cyan" },
  { to: "/system", label: "Health", icon: Activity, badge: 0, accent: "lime" },
];

const ACCENT_VAR: Record<NavItem["accent"], string> = {
  cyan: "var(--color-cyan)",
  magenta: "var(--color-magenta)",
  lime: "var(--color-lime)",
  orange: "var(--color-orange)",
};

export function Sidebar({ username }: { username?: string }) {
  async function handleLogout() {
    try {
      await api.logout();
    } catch {
      // ignore — cookie may already be invalid
    }
    window.location.href = "/login";
  }

  return (
    <aside className="w-[220px] shrink-0 h-full flex flex-col border-r border-[var(--color-line)] bg-[rgba(10,15,36,0.55)] backdrop-blur-sm relative z-10">
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
          return (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.to === "/"}
              className={({ isActive }) =>
                cn(
                  "group relative flex items-center gap-3 px-3 py-2 font-mono text-[11px] uppercase tracking-hud-tight transition-colors clip-tag",
                  isActive
                    ? "bg-[rgba(94,240,255,0.08)] text-[var(--color-fg)]"
                    : "text-[var(--color-dim)] hover:text-[var(--color-fg)] hover:bg-[rgba(120,255,220,0.04)]"
                )
              }
              style={({ isActive }: { isActive: boolean }) =>
                isActive
                  ? {
                      borderLeft: `2px solid ${accentColor}`,
                    }
                  : { borderLeft: "2px solid transparent" }
              }
            >
              {({ isActive }) => (
                <>
                  <Icon
                    size={14}
                    strokeWidth={1.6}
                  />
                  <span className="flex-1">{item.label}</span>
                  {item.badge !== undefined && item.badge > 0 && (
                    <span
                      className="font-display font-bold text-[10px] px-1.5"
                      style={{
                        color: accentColor,
                        background: `${accentColor}15`,
                      }}
                    >
                      {item.badge}
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
                </>
              )}
            </NavLink>
          );
        })}
      </nav>

      {/* user footer */}
      <div className="px-3 py-3 border-t border-[var(--color-line)]">
        <div className="flex items-center gap-2 px-2 py-2 clip-tag bg-[rgba(94,240,255,0.04)]">
          <div
            className="w-6 h-6 flex items-center justify-center font-display font-bold text-[10px]"
            style={{
              background: "linear-gradient(135deg, var(--color-cyan), var(--color-magenta))",
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
