import * as React from "react";
import { Cpu, MemoryStick, Thermometer, Bot } from "lucide-react";
import { useTopic } from "@/lib/useTopic";
import type { SystemStats } from "@/lib/api";

function parseStats(payload: unknown): SystemStats | null {
  if (typeof payload === "string") {
    try {
      return JSON.parse(payload) as SystemStats;
    } catch {
      return null;
    }
  }
  if (payload && typeof payload === "object") return payload as SystemStats;
  return null;
}

function tone(pct: number): string {
  if (pct >= 90) return "var(--color-danger)";
  if (pct >= 70) return "var(--color-orange)";
  return "var(--color-cyan)";
}

function tempTone(t: number): string {
  if (t >= 85) return "var(--color-danger)";
  if (t >= 70) return "var(--color-orange)";
  return "var(--color-cyan)";
}

interface RowProps {
  Icon: React.ComponentType<{ size?: number; strokeWidth?: number; style?: React.CSSProperties }>;
  label: string;
  value: string;
  pct?: number;
  accent: string;
}

function Row({ Icon, label, value, pct, accent }: RowProps) {
  return (
    <div className="flex items-center gap-2 px-2 py-1">
      <Icon size={11} strokeWidth={1.6} style={{ color: accent }} />
      <span className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight uppercase w-7">
        {label}
      </span>
      <div className="flex-1 h-1 bg-[rgba(255,255,255,0.05)] overflow-hidden relative">
        {pct !== undefined && (
          <div
            className="absolute inset-y-0 left-0 transition-all"
            style={{ width: `${Math.min(100, Math.max(0, pct))}%`, background: accent }}
          />
        )}
      </div>
      <span
        className="font-mono text-[10px] tabular-nums"
        style={{ color: accent, minWidth: 56, textAlign: "right" }}
      >
        {value}
      </span>
    </div>
  );
}

export function SidebarStats() {
  const [stats, setStats] = React.useState<SystemStats | null>(null);

  const handle = React.useCallback((payload: unknown, evt: { type: string }) => {
    if (evt.type !== "stats") return;
    const s = parseStats(payload);
    if (s) setStats(s);
  }, []);

  useTopic("system", handle);

  const cpu = stats?.cpu_pct ?? 0;
  const ramPct =
    stats && stats.ram_total_gb > 0 ? (stats.ram_used_gb / stats.ram_total_gb) * 100 : 0;
  const temp = stats?.temp_c ?? 0;
  const running = stats?.running_agents ?? 0;

  return (
    <div className="px-1 py-2 border-t border-[var(--color-line)] flex flex-col gap-0.5">
      <Row Icon={Cpu} label="CPU" value={`${cpu.toFixed(2)}%`} pct={cpu} accent={tone(cpu)} />
      <Row Icon={MemoryStick} label="RAM" value={`${ramPct.toFixed(2)}%`} pct={ramPct} accent={tone(ramPct)} />
      <Row Icon={Thermometer} label="TEMP" value={`${temp.toFixed(1)}°`} accent={tempTone(temp)} />
      <Row
        Icon={Bot}
        label="RUN"
        value={running > 0 ? `${running} live` : "idle"}
        accent={running > 0 ? "var(--color-orange)" : "var(--color-dim)"}
      />
    </div>
  );
}
