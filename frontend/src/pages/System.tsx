import * as React from "react";
import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import { Gauge } from "@/components/Gauge";
import {
  api,
  type SystemStats,
  type SystemService,
  type SystemProcess,
  type SystemConnections,
} from "@/lib/api";
import { useTopic } from "@/lib/useTopic";
import { wsClient } from "@/lib/wsClient";

const SLOW_POLL_MS = 8000; // services / processes / connections

function parseStatsPayload(payload: unknown): SystemStats | null {
  if (!payload) return null;
  if (typeof payload === "string") {
    try {
      const obj = JSON.parse(payload);
      return obj && typeof obj === "object" ? (obj as SystemStats) : null;
    } catch {
      return null;
    }
  }
  if (typeof payload === "object") return payload as SystemStats;
  return null;
}

function formatUptime(secs: number): string {
  if (!secs || secs < 0) return "—";
  const days = Math.floor(secs / 86400);
  const hours = Math.floor((secs % 86400) / 3600);
  const mins = Math.floor((secs % 3600) / 60);
  return `${days}D ${String(hours).padStart(2, "0")}H ${String(mins).padStart(2, "0")}M`;
}

function tempColor(temp: number): string {
  if (temp >= 80) return "var(--color-danger)";
  if (temp >= 65) return "var(--color-warn)";
  return "var(--color-lime)";
}

function diskColor(pct: number): string {
  if (pct >= 85) return "var(--color-danger)";
  if (pct >= 70) return "var(--color-warn)";
  return "var(--color-lime)";
}

export function System() {
  const [stats, setStats] = React.useState<SystemStats | null>(null);
  const [services, setServices] = React.useState<SystemService[]>([]);
  const [processes, setProcesses] = React.useState<SystemProcess[]>([]);
  const [connections, setConnections] = React.useState<SystemConnections | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [actionState, setActionState] = React.useState<{
    name: string;
    action: "start" | "stop" | "restart";
    busy: boolean;
  } | null>(null);
  const [confirm, setConfirm] = React.useState<{ name: string; action: "stop" | "restart" } | null>(
    null
  );

  // ─── live stats: initial HTTP snapshot, then WS updates ──────────
  const fetchStats = React.useCallback(async () => {
    try {
      const s = await api.systemStats();
      setStats(s);
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "stats fail");
    }
  }, []);

  // initial fetch
  React.useEffect(() => {
    void fetchStats();
  }, [fetchStats]);

  const handleStatsMessage = React.useCallback(
    (payload: unknown, evt: { type: string }) => {
      if (evt.type !== "stats") return;
      const stats = parseStatsPayload(payload);
      if (!stats) return;
      setStats(stats);
      setError(null);
    },
    [],
  );

  const { status: wsStatus } = useTopic("system", handleStatsMessage);

  // On reconnect, refresh once to reconcile anything missed while offline.
  React.useEffect(() => {
    if (wsStatus === "open") {
      void fetchStats();
    }
  }, [wsStatus, fetchStats]);

  // ─── services / processes / connections: HTTP poll ──────────────
  const refreshSlow = React.useCallback(async () => {
    try {
      const [svc, procs, conn] = await Promise.all([
        api.systemServices().catch(() => [] as SystemService[]),
        api.systemProcesses(10, "cpu").catch(() => [] as SystemProcess[]),
        api.systemConnections().catch(() => null),
      ]);
      setServices(svc);
      setProcesses(procs);
      if (conn) setConnections(conn);
    } catch {
      /* keep last-known good */
    }
  }, []);

  React.useEffect(() => {
    void refreshSlow();
    const id = window.setInterval(refreshSlow, SLOW_POLL_MS);
    return () => window.clearInterval(id);
  }, [refreshSlow]);

  // ─── actions ────────────────────────────────────────────────────
  async function runAction(name: string, action: "start" | "stop" | "restart") {
    setActionState({ name, action, busy: true });
    try {
      await wsClient.request("service_action", { name, op: action });
      // refresh services after a tick — systemd takes a moment
      window.setTimeout(() => void refreshSlow(), 500);
    } catch (err) {
      setError(err instanceof Error ? err.message : `failed ${action} ${name}`);
    } finally {
      setActionState((s) => (s ? { ...s, busy: false } : null));
      window.setTimeout(() => setActionState(null), 1200);
      setConfirm(null);
    }
  }

  function requestAction(name: string, action: "start" | "stop" | "restart") {
    if (action === "start") {
      void runAction(name, action);
      return;
    }
    setConfirm({ name, action });
  }

  // ─── derived ────────────────────────────────────────────────────
  const ramPct =
    stats && stats.ram_total_gb > 0
      ? (stats.ram_used_gb / stats.ram_total_gb) * 100
      : 0;
  const primaryDisk = stats?.disks?.[0];
  const diskPct = primaryDisk?.used_pct ?? 0;
  const tempC = stats?.temp_c ?? 0;
  const uptime = stats?.uptime_s ?? 0;
  const uptimeDays = Math.floor(uptime / 86400);

  const tone = error
    ? ("danger" as const)
    : wsStatus === "open"
    ? ("ok" as const)
    : ("warn" as const);

  const transportLabel =
    wsStatus === "open"
      ? "ws · live"
      : wsStatus === "connecting"
      ? "ws · connecting…"
      : "ws · reconnect…";

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "System Manager" }]}
        status={{
          label: error
            ? "ERROR"
            : wsStatus === "open"
            ? "NOMINAL"
            : "CONNECTING",
          tone,
        }}
        right={
          <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
            {transportLabel}
          </span>
        }
      />

      <div className="flex-1 min-h-0 p-2 overflow-y-auto sm:p-4">
        {/* ─── top stats row: 5 cards with gauges ─── */}
        <div className="grid grid-cols-2 gap-3 mb-4 sm:grid-cols-3 xl:grid-cols-5">
          <StatCard accent="cyan">
            <Gauge
              value={stats?.cpu_pct ?? 0}
              label="CPU"
              size={130}
              color="var(--color-cyan)"
              sub={
                stats?.load_avg
                  ? `load ${stats.load_avg[0].toFixed(2)}`
                  : "load —"
              }
            />
          </StatCard>

          <StatCard accent="lime">
            <Gauge
              value={ramPct}
              label="RAM"
              size={130}
              color="var(--color-lime)"
              sub={
                stats
                  ? `${stats.ram_used_gb.toFixed(1)}/${stats.ram_total_gb.toFixed(0)} GB`
                  : "—"
              }
            />
          </StatCard>

          <StatCard accent="orange">
            <Gauge
              value={diskPct}
              label="DISK"
              size={130}
              color={diskColor(diskPct)}
              sub={
                primaryDisk
                  ? `${primaryDisk.used_gb.toFixed(0)}/${primaryDisk.total_gb.toFixed(0)} GB`
                  : "—"
              }
            />
          </StatCard>

          <StatCard accent="magenta">
            <Gauge
              value={tempC}
              max={100}
              unit="°C"
              label="TEMP"
              size={130}
              color={tempColor(tempC)}
              sub={tempC > 70 ? "high" : "ok"}
            />
          </StatCard>

          <StatCard accent="cyan">
            <div className="flex flex-col items-center justify-center h-[130px]">
              <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud uppercase">
                UPTIME
              </div>
              <div className="font-display text-[24px] font-bold text-[var(--color-fg)] mt-1 leading-none">
                {uptimeDays}
                <span className="text-[var(--color-dim)] text-[14px] ml-1">D</span>
              </div>
              <div className="font-mono text-[10px] text-[var(--color-cyan)] tracking-hud mt-1">
                {formatUptime(uptime)}
              </div>
              {stats?.load_avg && (
                <div className="font-mono text-[9px] text-[var(--color-dim)] mt-1">
                  load {stats.load_avg.map((v) => v.toFixed(2)).join(" · ")}
                </div>
              )}
            </div>
          </StatCard>
        </div>

        {/* ─── main grid: services (big left) + connections + processes (right) ─── */}
        <div className="grid grid-cols-1 gap-4 xl:grid-cols-3">
          {/* services panel — spans 2 cols */}
          <div className="min-h-[420px] xl:col-span-2">
            <HudPanel
              title="servicios systemd"
              sub={`${services.filter((s) => s.state === "active").length}/${services.length} activos`}
              accent="cyan"
              className="h-full"
            >
              {services.length === 0 ? (
                <div className="flex-1 flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)] tracking-hud-tight">
                  ▸ sin datos · esperando backend…
                </div>
              ) : (
                <div className="flex-1 min-h-0 overflow-x-auto overflow-y-auto pr-1">
                  {services.map((s) => (
                    <ServiceRow
                      key={s.name}
                      svc={s}
                      busy={
                        actionState?.name === s.name && actionState.busy
                      }
                      onAction={(action) => requestAction(s.name, action)}
                    />
                  ))}
                </div>
              )}

              {confirm && (
                <ConfirmBar
                  name={confirm.name}
                  action={confirm.action}
                  onConfirm={() => void runAction(confirm.name, confirm.action)}
                  onCancel={() => setConfirm(null)}
                />
              )}
            </HudPanel>
          </div>

          {/* right column: connections + processes */}
          <div className="grid gap-4 min-h-[420px] xl:grid-rows-[auto_1fr]">
            <HudPanel title="conexiones" sub="wa · ws · tunnels" accent="magenta">
              <ConnectionsPanel conn={connections} />
            </HudPanel>

            <HudPanel
              title="top procesos"
              sub={`${processes.length} · sort cpu`}
              accent="orange"
              className="min-h-0"
            >
              {processes.length === 0 ? (
                <div className="flex-1 flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)]">
                  ▸ sin datos
                </div>
              ) : (
                <div className="flex-1 min-h-0 overflow-y-auto pr-1">
                  {processes.map((p) => (
                    <ProcessRow key={p.pid} proc={p} maxCpu={Math.max(...processes.map((x) => x.cpu_pct), 1)} />
                  ))}
                </div>
              )}
            </HudPanel>
          </div>
        </div>

        {/* ─── disks detail row (only if more than one) ─── */}
        {stats && stats.disks && stats.disks.length > 1 && (
          <div className="mt-4">
            <HudPanel title="almacenamiento" sub={`${stats.disks.length} volúmenes`} accent="orange">
              <div className="grid grid-cols-1 gap-3 py-2 sm:grid-cols-2">
                {stats.disks.map((d) => (
                  <DiskRow key={d.mount} mount={d.mount} used={d.used_gb} total={d.total_gb} pct={d.used_pct} />
                ))}
              </div>
            </HudPanel>
          </div>
        )}

        {error && (
          <div
            className="mt-4 px-3 py-2 font-mono text-[10px] clip-hud-sm"
            style={{
              background: "rgba(255, 92, 122, 0.08)",
              border: "1px solid rgba(255, 92, 122, 0.45)",
              color: "var(--color-danger)",
            }}
          >
            ✗ {error}
          </div>
        )}
      </div>
    </div>
  );
}

// ─── subcomponents ──────────────────────────────────────────────────

function StatCard({
  accent,
  children,
}: {
  accent: "cyan" | "magenta" | "lime" | "orange" | "danger";
  children: React.ReactNode;
}) {
  return (
    <HudPanel accent={accent} className="min-h-[170px]">
      <div className="flex-1 flex items-center justify-center">{children}</div>
    </HudPanel>
  );
}

function ServiceRow({
  svc,
  busy,
  onAction,
}: {
  svc: SystemService;
  busy: boolean;
  onAction: (action: "start" | "stop" | "restart") => void;
}) {
  const stateColor =
    svc.state === "active"
      ? "var(--color-lime)"
      : svc.state === "failed"
      ? "var(--color-danger)"
      : svc.state === "activating" || svc.state === "deactivating"
      ? "var(--color-warn)"
      : "var(--color-dim)";

  const isActive = svc.state === "active";
  const isInactive = svc.state === "inactive" || svc.state === "failed";
  const since = svc.since
    ? new Date(svc.since * 1000).toISOString().slice(0, 19).replace("T", " ")
    : "";

  return (
    <div
      className="grid min-w-[520px] items-center gap-3 py-2 border-b border-[var(--color-line)] font-mono text-[11px]"
      style={{ gridTemplateColumns: "12px 1fr auto auto auto" }}
    >
      <span
        className="w-1.5 h-1.5 rounded-full"
        style={{
          background: stateColor,
          boxShadow: isActive ? `0 0 6px ${stateColor}` : "none",
        }}
      />
      <div className="min-w-0">
        <div className="text-[var(--color-fg)] truncate">{svc.name}</div>
        <div className="text-[var(--color-dim)] text-[9px] mt-0.5 tracking-hud-tight">
          <span style={{ color: stateColor }}>● {svc.state}</span>
          {since && <span className="ml-2">since {since}</span>}
          {typeof svc.cpu_pct === "number" && (
            <span className="ml-2 text-[var(--color-cyan)]">{svc.cpu_pct.toFixed(1)}% cpu</span>
          )}
          {typeof svc.mem_mb === "number" && (
            <span className="ml-2 text-[var(--color-magenta)]">{Math.round(svc.mem_mb)}M</span>
          )}
        </div>
      </div>

      {/* restart */}
      <ActionButton
        accent="var(--color-cyan)"
        label="↻ restart"
        disabled={busy}
        onClick={() => onAction("restart")}
      />
      {/* stop / start */}
      {isActive ? (
        <ActionButton
          accent="var(--color-danger)"
          label="⏹ stop"
          disabled={busy}
          onClick={() => onAction("stop")}
        />
      ) : (
        <ActionButton
          accent="var(--color-lime)"
          label="▶ start"
          disabled={busy || !isInactive}
          onClick={() => onAction("start")}
        />
      )}
      <span className="font-mono text-[9px] text-[var(--color-dim)] w-[24px] text-right">
        {busy ? "…" : ""}
      </span>
    </div>
  );
}

function ActionButton({
  accent,
  label,
  disabled,
  onClick,
}: {
  accent: string;
  label: string;
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="px-2 py-1 font-mono text-[9px] tracking-hud uppercase clip-tag transition-opacity"
      style={{
        color: accent,
        border: `1px solid ${accent}`,
        background: `${accent}12`,
        opacity: disabled ? 0.4 : 1,
        cursor: disabled ? "not-allowed" : "pointer",
      }}
    >
      {label}
    </button>
  );
}

function ConfirmBar({
  name,
  action,
  onConfirm,
  onCancel,
}: {
  name: string;
  action: "stop" | "restart";
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const accent =
    action === "stop" ? "var(--color-danger)" : "var(--color-warn)";
  return (
    <div
      className="mt-2 px-3 py-2 clip-hud-sm flex items-center justify-between gap-2 font-mono text-[10px]"
      style={{
        background: `${accent}12`,
        border: `1px solid ${accent}`,
        color: accent,
      }}
    >
      <span>
        ⚠ confirmar {action.toUpperCase()} sobre <b className="text-[var(--color-fg)]">{name}</b>?
      </span>
      <div className="flex gap-2">
        <button
          onClick={onConfirm}
          className="px-2 py-0.5 clip-tag font-mono text-[10px] tracking-hud"
          style={{
            color: "var(--color-bg)",
            background: accent,
            cursor: "pointer",
          }}
        >
          confirmar
        </button>
        <button
          onClick={onCancel}
          className="px-2 py-0.5 clip-tag font-mono text-[10px] tracking-hud"
          style={{
            color: "var(--color-dim)",
            border: "1px solid var(--color-line)",
            cursor: "pointer",
          }}
        >
          cancelar
        </button>
      </div>
    </div>
  );
}

function ConnectionsPanel({ conn }: { conn: SystemConnections | null }) {
  if (!conn) {
    return (
      <div className="font-mono text-[11px] text-[var(--color-dim)] py-2">
        ▸ sin datos · esperando backend…
      </div>
    );
  }

  const waColor =
    conn.wa === "connected"
      ? "var(--color-lime)"
      : conn.wa === "pairing"
      ? "var(--color-warn)"
      : "var(--color-danger)";

  return (
    <div className="flex flex-col gap-2 py-1">
      <ConnRow
        label="whatsapp"
        value={conn.wa}
        accent={waColor}
        dot
      />
      <ConnRow
        label="ws clients"
        value={String(conn.ws_clients)}
        accent="var(--color-cyan)"
      />
      <div className="mt-1">
        <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud uppercase mb-1">
          tunnels
        </div>
        {conn.tunnels.length === 0 ? (
          <div className="font-mono text-[10px] text-[var(--color-dim)]">— ninguno —</div>
        ) : (
          conn.tunnels.map((t) => (
            <ConnRow
              key={t.name}
              label={t.name}
              value={t.state}
              accent={t.state === "up" ? "var(--color-lime)" : "var(--color-danger)"}
              dot
            />
          ))
        )}
      </div>
    </div>
  );
}

function ConnRow({
  label,
  value,
  accent,
  dot,
}: {
  label: string;
  value: string;
  accent: string;
  dot?: boolean;
}) {
  return (
    <div
      className="px-3 py-1.5 clip-hud-sm flex items-center justify-between"
      style={{
        background: `${accent}0d`,
        border: `1px solid ${accent}40`,
      }}
    >
      <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud uppercase">
        {label}
      </span>
      <span className="flex items-center gap-2 font-display text-[12px] font-semibold tracking-hud-tight uppercase" style={{ color: accent }}>
        {dot && (
          <span
            className="w-1.5 h-1.5 rounded-full"
            style={{ background: accent, boxShadow: `0 0 6px ${accent}` }}
          />
        )}
        {value}
      </span>
    </div>
  );
}

function ProcessRow({ proc, maxCpu }: { proc: SystemProcess; maxCpu: number }) {
  const pct = (proc.cpu_pct / maxCpu) * 100;
  const c =
    proc.cpu_pct > 50
      ? "var(--color-danger)"
      : proc.cpu_pct > 20
      ? "var(--color-warn)"
      : "var(--color-cyan)";
  return (
    <div className="py-1.5 border-b border-[var(--color-line)]">
      <div className="flex items-center justify-between font-mono text-[10px] mb-1">
        <span className="text-[var(--color-fg)] truncate flex-1">
          <span className="text-[var(--color-dim)] mr-2">{proc.pid}</span>
          {proc.name}
        </span>
        <span className="text-[var(--color-magenta)] ml-2 shrink-0">
          {Math.round(proc.mem_mb)}M
        </span>
        <span className="font-display font-semibold ml-2 shrink-0" style={{ color: c }}>
          {proc.cpu_pct.toFixed(1)}%
        </span>
      </div>
      <div
        className="h-1 clip-bar overflow-hidden"
        style={{ background: "rgba(255,255,255,0.05)" }}
      >
        <div
          style={{
            width: `${Math.max(2, Math.min(100, pct))}%`,
            height: "100%",
            background: `linear-gradient(90deg, ${c}, ${c}aa)`,
            boxShadow: `0 0 6px ${c}`,
            transition: "width 600ms ease",
          }}
        />
      </div>
    </div>
  );
}

function DiskRow({
  mount,
  used,
  total,
  pct,
}: {
  mount: string;
  used: number;
  total: number;
  pct: number;
}) {
  const c = diskColor(pct);
  return (
    <div>
      <div className="flex justify-between font-mono text-[10px] mb-1">
        <span className="text-[var(--color-fg)]">
          {mount}
          <span className="text-[var(--color-dim)] ml-2">
            {used.toFixed(1)} / {total.toFixed(1)} GB
          </span>
        </span>
        <span className="font-display font-semibold" style={{ color: c }}>
          {pct.toFixed(0)}%
        </span>
      </div>
      <div
        className="h-1.5 clip-bar overflow-hidden"
        style={{ background: "rgba(255,255,255,0.05)" }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: "100%",
            background: `linear-gradient(90deg, ${c}, ${c}cc)`,
            boxShadow: `0 0 6px ${c}`,
          }}
        />
      </div>
    </div>
  );
}
