import * as React from "react";
import { Outlet, useNavigate } from "react-router-dom";
import { MobileNav, Sidebar } from "@/components/Sidebar";
import { api, ApiError, type Me } from "@/lib/api";
import { NotificationProvider } from "@/lib/notifications";
import { StreamsProvider } from "@/lib/streamsStore";

interface AuthState {
  status: "loading" | "ok" | "error";
  me: Me | null;
  error: string | null;
}

/**
 * AppShell — protected layout. Verifies the cookie via /api/auth/me
 * on mount; redirects to /login on 401. Renders sidebar + outlet.
 */
export function AppShell() {
  const navigate = useNavigate();
  const [auth, setAuth] = React.useState<AuthState>({
    status: "loading",
    me: null,
    error: null,
  });

  React.useEffect(() => {
    let alive = true;
    (async () => {
      try {
        const me = await api.me();
        if (!alive) return;
        setAuth({ status: "ok", me, error: null });
      } catch (err) {
        if (!alive) return;
        if (err instanceof ApiError && err.status === 401) {
          navigate("/login", { replace: true });
          return;
        }
        setAuth({
          status: "error",
          me: null,
          error: err instanceof Error ? err.message : "error de red",
        });
      }
    })();
    return () => {
      alive = false;
    };
  }, [navigate]);

  if (auth.status === "loading") {
    return (
      <div className="min-h-screen w-full grid place-items-center font-mono text-[12px] text-[var(--color-dim)] tracking-hud">
        <div>▸ inicializando sesión…</div>
      </div>
    );
  }

  if (auth.status === "error") {
    return (
      <div className="min-h-screen w-full grid place-items-center text-center px-6">
        <div>
          <div className="font-display text-[20px] tracking-hud text-[var(--color-danger)] uppercase mb-2">
            ✗ sin conexión al daemon
          </div>
          <div className="font-mono text-[12px] text-[var(--color-dim)] max-w-md">
            {auth.error ?? "no se pudo contactar el backend en :8093"}
          </div>
        </div>
      </div>
    );
  }

  return (
    <NotificationProvider>
      <StreamsProvider>
        <div className="flex h-screen w-screen overflow-hidden">
          <div className="hud-grid" />
          <div className="hud-scan" />
          <Sidebar username={auth.me?.username} />
          <main className="flex-1 flex flex-col min-w-0 relative z-10 pb-[72px] md:pb-0">
            <Outlet />
          </main>
          <MobileNav username={auth.me?.username} />
        </div>
      </StreamsProvider>
    </NotificationProvider>
  );
}
