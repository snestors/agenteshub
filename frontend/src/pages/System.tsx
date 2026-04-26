import * as React from "react";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { api } from "@/lib/api";

interface HealthState {
  ok: boolean;
  ts: number | null;
  error: string | null;
  lastChecked: number;
}

export function System() {
  const [health, setHealth] = React.useState<HealthState>({
    ok: false,
    ts: null,
    error: null,
    lastChecked: 0,
  });

  React.useEffect(() => {
    let alive = true;
    async function check() {
      try {
        const res = await api.health();
        if (!alive) return;
        setHealth({
          ok: res.ok,
          ts: res.ts,
          error: null,
          lastChecked: Date.now(),
        });
      } catch (err) {
        if (!alive) return;
        setHealth({
          ok: false,
          ts: null,
          error: err instanceof Error ? err.message : "error de red",
          lastChecked: Date.now(),
        });
      }
    }
    void check();
    const id = window.setInterval(check, 5000);
    return () => {
      alive = false;
      window.clearInterval(id);
    };
  }, []);

  const tone = health.ok ? "ok" : "danger";

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Health" }]}
        status={{ label: health.ok ? "NOMINAL" : "OFFLINE", tone }}
      />

      <div className="flex-1 min-h-0 p-4 overflow-hidden grid grid-cols-2 grid-rows-2 gap-4">
        <HudPanel title="daemon" sub="GET /healthz" accent="lime">
          <div className="flex-1 flex flex-col gap-3 py-2">
            <Stat
              label="estado"
              value={health.ok ? "ONLINE" : "OFFLINE"}
              accent={health.ok ? "var(--color-lime)" : "var(--color-danger)"}
            />
            <Stat
              label="server ts"
              value={health.ts ? new Date(health.ts * 1000).toISOString() : "—"}
              accent="var(--color-cyan)"
            />
            <Stat
              label="último check"
              value={
                health.lastChecked
                  ? `${Math.round((Date.now() - health.lastChecked) / 1000)}s atrás`
                  : "—"
              }
              accent="var(--color-fg)"
            />
            {health.error && (
              <div
                className="px-3 py-2 font-mono text-[10px] clip-hud-sm"
                style={{
                  background: "rgba(255, 92, 122, 0.08)",
                  border: "1px solid rgba(255, 92, 122, 0.45)",
                  color: "var(--color-danger)",
                }}
              >
                ✗ {health.error}
              </div>
            )}
          </div>
        </HudPanel>

        <HudPanel title="próximos paneles" sub="v1+" accent="cyan">
          <div className="font-mono text-[11px] text-[var(--color-dim)] leading-relaxed py-2">
            <div>▸ vitals (cpu / ram / gpu)</div>
            <div>▸ uplink + ancho de banda</div>
            <div>▸ servicios (systemd)</div>
            <div>▸ logs en vivo (journalctl -f)</div>
            <div>▸ contenedores (docker ps)</div>
            <div>▸ cola de tareas</div>
          </div>
        </HudPanel>

        <HudPanel title="api" sub="endpoints disponibles" accent="orange">
          <div className="font-mono text-[10px] leading-[1.7] text-[var(--color-fg)]">
            <Endpoint method="GET " path="/healthz" />
            <Endpoint method="POST" path="/api/auth/login" />
            <Endpoint method="POST" path="/api/auth/totp" />
            <Endpoint method="POST" path="/api/auth/logout" />
            <Endpoint method="POST" path="/api/auth/refresh" />
            <Endpoint method="GET " path="/api/auth/me" />
            <Endpoint method="GET " path="/api/messages" />
            <Endpoint method="POST" path="/api/messages" />
          </div>
        </HudPanel>

        <HudPanel title="info" sub="binario único" accent="magenta">
          <div className="font-mono text-[11px] text-[var(--color-dim)] leading-relaxed py-2">
            <div className="text-[var(--color-fg)] mb-2 font-display tracking-hud uppercase text-[12px]">
              AgentHub Go v0.1
            </div>
            Daemon Go que centraliza la vida digital en la mini PC: 1 agente
            principal (WA + Browser), N mini-agentes especializados, N proyectos
            de coding con sesiones independientes.
          </div>
        </HudPanel>
      </div>
    </div>
  );
}

function Stat({
  label,
  value,
  accent,
}: {
  label: string;
  value: string;
  accent: string;
}) {
  return (
    <div
      className="px-3 py-2 clip-hud-sm border"
      style={{
        background: `${accent}0d`,
        borderColor: `${accent}40`,
      }}
    >
      <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud uppercase">
        {label}
      </div>
      <div
        className="font-display font-semibold text-[14px] mt-0.5 tracking-hud-tight"
        style={{ color: accent }}
      >
        {value}
      </div>
    </div>
  );
}

function Endpoint({ method, path }: { method: string; path: string }) {
  const colors: Record<string, string> = {
    "GET ": "var(--color-cyan)",
    POST: "var(--color-magenta)",
  };
  return (
    <div className="flex gap-3 py-0.5 border-b border-[var(--color-line)]">
      <span style={{ color: colors[method] ?? "var(--color-fg)" }} className="font-semibold">
        {method}
      </span>
      <span className="text-[var(--color-fg)]">{path}</span>
    </div>
  );
}
