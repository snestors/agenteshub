import * as React from "react";
import { TrendingUp, RefreshCw } from "lucide-react";

import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";
import {
  api,
  type UsageBucket,
  type RealtimeResponse,
  type RealtimeProvider,
  ApiError,
} from "@/lib/api";

// ─── formatting helpers ───────────────────────────────────────────────────────

function fmtUsd(v: number): string {
  if (v === 0) return "$0.00";
  if (v < 0.01) return `$${v.toFixed(4)}`;
  return `$${v.toFixed(2)}`;
}

function fmtTokensCompact(v: number): string {
  if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
  if (v >= 1_000) return `${(v / 1_000).toFixed(1)}K`;
  return String(v);
}

function fmtDelta(current: number, previous: number, isUsd = false): string {
  if (previous === 0) return "";
  const diff = current - previous;
  const sign = diff >= 0 ? "+" : "";
  if (isUsd) return `${sign}${fmtUsd(diff)}`;
  return `${sign}${fmtTokensCompact(diff)}`;
}

function fmtCountdown(resetAt: number): string {
  const now = Math.floor(Date.now() / 1000);
  const diff = resetAt - now;
  if (diff <= 0) return "limit reset";
  const days = Math.floor(diff / 86400);
  const hours = Math.floor((diff % 86400) / 3600);
  const mins = Math.floor((diff % 3600) / 60);
  if (days > 0) return `en ${days}d ${hours}h`;
  if (hours > 0) return `en ${hours}h ${String(mins).padStart(2, "0")}m`;
  return `en ${mins}m`;
}

function fmtStamp(ts: number): string {
  return new Date(ts * 1000).toLocaleString("es-PE", {
    day: "2-digit",
    month: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function barColor(pct: number): string {
  if (pct > 90) return "var(--color-danger)";
  if (pct > 75) return "var(--color-warn)";
  if (pct > 50) return "var(--color-cyan)";
  return "var(--color-lime)";
}

// ─── sub-components ───────────────────────────────────────────────────────────

/** Skeleton block used during loading states */
function Skeleton({ className, style }: { className?: string; style?: React.CSSProperties }) {
  return (
    <div
      className={`rounded-sm animate-pulse bg-[rgba(255,255,255,0.05)] ${className ?? ""}`}
      style={style}
    />
  );
}

/** Small stat card — top band */
interface StatCardProps {
  label: string;
  value: string;
  delta?: string;
  deltaPositiveBad?: boolean;
  accent: string;
  loading?: boolean;
}

function StatCard({ label, value, delta, deltaPositiveBad, accent, loading }: StatCardProps) {
  const deltaIsPositive = delta?.startsWith("+");
  const deltaColor =
    delta === ""
      ? "var(--color-dim)"
      : deltaPositiveBad
      ? deltaIsPositive
        ? "var(--color-danger)"
        : "var(--color-lime)"
      : deltaIsPositive
      ? "var(--color-lime)"
      : "var(--color-danger)";

  return (
    <div
      className="relative clip-hud-sm p-[2px]"
      style={{
        background: `linear-gradient(135deg, ${accent}50, ${accent}10 50%, ${accent}30)`,
      }}
    >
      <div className="clip-hud-sm bg-[var(--color-bg-2)] px-4 py-3 flex flex-col gap-1">
        <div className="font-mono text-[10px] uppercase tracking-hud text-[var(--color-dim)]">
          {label}
        </div>
        {loading ? (
          <>
            <Skeleton className="h-8 w-24 mt-1" />
            <Skeleton className="h-3 w-16 mt-1" />
          </>
        ) : (
          <>
            <div
              className="font-display text-[24px] font-bold leading-none"
              style={{ color: accent }}
            >
              {value}
            </div>
            {delta !== undefined && (
              <div className="font-mono text-[10px]" style={{ color: deltaColor }}>
                {delta || "—"}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

/** Progress bar for realtime windows */
function WindowBar({
  label,
  pct,
  resetAt,
  usedTokens,
  limitTokens,
}: {
  label: string;
  pct: number;
  resetAt: number;
  usedTokens: number;
  limitTokens: number;
}) {
  const clampedPct = Math.min(100, Math.max(0, pct));
  const color = barColor(clampedPct);
  return (
    <div className="flex flex-col gap-1">
      <div className="flex items-center justify-between font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)]">
        <span>{label}</span>
        <span style={{ color }}>{clampedPct.toFixed(0)}%</span>
      </div>
      <div
        className="h-1.5 overflow-hidden clip-bar"
        style={{ background: "rgba(255,255,255,0.06)" }}
      >
        <div
          style={{
            width: `${clampedPct}%`,
            height: "100%",
            background: `linear-gradient(90deg, ${color}, ${color}cc)`,
            boxShadow: `0 0 6px ${color}`,
            transition: "width 600ms ease",
          }}
        />
      </div>
      <div className="flex items-center justify-between font-mono text-[9px] text-[var(--color-dim)]">
        <span>
          {fmtTokensCompact(usedTokens)} / {fmtTokensCompact(limitTokens)} tok
        </span>
        <span>{fmtCountdown(resetAt)}</span>
      </div>
    </div>
  );
}

/** Card for a single realtime provider (Claude or Codex) */
function ProviderCard({
  name,
  provider,
  loading,
}: {
  name: string;
  provider: RealtimeProvider | undefined;
  loading: boolean;
}) {
  const accent =
    name === "CLAUDE" ? "var(--color-cyan)" : "var(--color-magenta)";
  const accentHex = name === "CLAUDE" ? "#5ef0ff" : "#ff4ed6";

  if (loading) {
    return (
      <div
        className="relative clip-hud-sm p-[2px]"
        style={{
          background: `linear-gradient(135deg, ${accentHex}40, ${accentHex}10)`,
        }}
      >
        <div className="clip-hud-sm bg-[var(--color-bg-2)] px-3 py-3 flex flex-col gap-3">
          <div className="flex items-center justify-between">
            <Skeleton className="h-4 w-16" />
            <Skeleton className="h-4 w-10" />
          </div>
          <Skeleton className="h-3 w-full" />
          <Skeleton className="h-3 w-full" />
        </div>
      </div>
    );
  }

  if (!provider) {
    return (
      <div
        className="relative clip-hud-sm p-[2px]"
        style={{
          background: `linear-gradient(135deg, ${accentHex}40, ${accentHex}10)`,
        }}
      >
        <div className="clip-hud-sm bg-[var(--color-bg-2)] px-3 py-3 flex flex-col gap-2">
          <div className="flex items-center gap-2">
            <span
              className="font-display text-[12px] font-bold tracking-hud uppercase"
              style={{ color: accent }}
            >
              {name}
            </span>
            <span
              className="px-2 py-0.5 clip-tag font-mono text-[9px] uppercase tracking-hud-tight"
              style={{
                color: "var(--color-warn)",
                border: "1px solid rgba(255,184,108,0.45)",
                background: "rgba(255,184,108,0.08)",
              }}
            >
              no auth
            </span>
          </div>
          <div className="font-mono text-[10px] text-[var(--color-dim)]">
            realtime no disponible — OAuth login requerido
          </div>
        </div>
      </div>
    );
  }

  return (
    <div
      className="relative clip-hud-sm p-[2px]"
      style={{
        background: `linear-gradient(135deg, ${accentHex}40, ${accentHex}10)`,
      }}
    >
      <div className="clip-hud-sm bg-[var(--color-bg-2)] px-3 py-3 flex flex-col gap-3">
        {/* error banner */}
        {provider.error && (
          <div
            className="px-2 py-1 clip-hud-sm font-mono text-[9px]"
            style={{
              color: "var(--color-warn)",
              border: "1px solid rgba(255,184,108,0.45)",
              background: "rgba(255,184,108,0.08)",
            }}
          >
            ⚠ {provider.error}
            {provider.fetched_at > 0 && (
              <span className="ml-2 text-[var(--color-dim)]">
                última carga {fmtStamp(provider.fetched_at)}
              </span>
            )}
          </div>
        )}

        {/* header: name + plan */}
        <div className="flex items-center justify-between">
          <span
            className="font-display text-[12px] font-bold tracking-hud uppercase"
            style={{ color: accent }}
          >
            {name}
          </span>
          <div className="flex items-center gap-2">
            {provider.plan && (
              <span
                className="px-2 py-0.5 clip-tag font-mono text-[9px] uppercase tracking-hud-tight"
                style={{
                  color: accent,
                  border: `1px solid ${accentHex}50`,
                  background: `${accentHex}10`,
                }}
              >
                {provider.plan}
              </span>
            )}
            {provider.account && (
              <span className="font-mono text-[9px] text-[var(--color-dim)] truncate max-w-[120px]">
                {provider.account}
              </span>
            )}
          </div>
        </div>

        {/* session window */}
        {provider.session && (
          <WindowBar
            label="session window"
            pct={provider.session.percent_used}
            resetAt={provider.session.reset_at}
            usedTokens={provider.session.used_tokens}
            limitTokens={provider.session.limit_tokens}
          />
        )}

        {/* weekly window */}
        {provider.weekly && (
          <WindowBar
            label="weekly limit"
            pct={provider.weekly.percent_used}
            resetAt={provider.weekly.reset_at}
            usedTokens={provider.weekly.used_tokens}
            limitTokens={provider.weekly.limit_tokens}
          />
        )}

        {/* credits (Codex) */}
        {provider.credits && (
          <div className="flex flex-col gap-1">
            <div className="flex items-center justify-between font-mono text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)]">
              <span>credits</span>
              <span style={{ color: accent }}>
                ${provider.credits.balance.toFixed(2)}
              </span>
            </div>
            <div
              className="h-1.5 overflow-hidden clip-bar"
              style={{ background: "rgba(255,255,255,0.06)" }}
            >
              {provider.credits.total > 0 && (
                <div
                  style={{
                    width: `${Math.min(100, (provider.credits.used / provider.credits.total) * 100)}%`,
                    height: "100%",
                    background: `linear-gradient(90deg, ${accent}, ${accentHex}cc)`,
                    boxShadow: `0 0 6px ${accent}`,
                  }}
                />
              )}
            </div>
            <div className="font-mono text-[9px] text-[var(--color-dim)]">
              {fmtUsd(provider.credits.used)} usado / {fmtUsd(provider.credits.total)} total
            </div>
          </div>
        )}

        {/* no windows placeholder */}
        {!provider.session && !provider.weekly && !provider.credits && (
          <div className="font-mono text-[10px] text-[var(--color-dim)]">
            sin datos de ventana disponibles
          </div>
        )}
      </div>
    </div>
  );
}

/** Model breakdown table */
function ModelTable({ buckets, loading }: { buckets: UsageBucket[]; loading: boolean }) {
  const sorted = React.useMemo(
    () => [...buckets].sort((a, b) => b.cost_usd - a.cost_usd),
    [buckets],
  );

  if (loading) {
    return (
      <div className="flex flex-col gap-2 pt-1">
        {[1, 2, 3].map((i) => (
          <Skeleton key={i} className="h-7 w-full" />
        ))}
      </div>
    );
  }

  if (sorted.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)]">
        ▸ sin datos en 30d
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="min-w-[560px] w-full font-mono text-[11px]">
        <thead>
          <tr
            className="text-left text-[9px] uppercase tracking-hud-tight text-[var(--color-dim)] border-b"
            style={{ borderColor: "var(--color-line)" }}
          >
            <th className="px-2 py-1.5">modelo</th>
            <th className="px-2 py-1.5 text-right">eventos</th>
            <th className="px-2 py-1.5 text-right">input</th>
            <th className="px-2 py-1.5 text-right">output</th>
            <th className="px-2 py-1.5 text-right">cache</th>
            <th className="px-2 py-1.5 text-right">costo</th>
          </tr>
        </thead>
        <tbody>
          {sorted.map((b, idx) => (
            <tr
              key={b.key}
              className="align-middle border-b"
              style={{ borderColor: "var(--color-line)" }}
            >
              <td className="px-2 py-1.5">
                <div className="flex items-center gap-2">
                  {idx < 3 && (
                    <span
                      className="w-1.5 h-1.5 rounded-full shrink-0"
                      style={{
                        background: "var(--color-lime)",
                        boxShadow: "0 0 4px var(--color-lime)",
                      }}
                    />
                  )}
                  <span className="text-[var(--color-fg)] truncate">{b.key}</span>
                </div>
              </td>
              <td className="px-2 py-1.5 text-right text-[var(--color-dim)]">
                {b.events.toLocaleString()}
              </td>
              <td className="px-2 py-1.5 text-right text-[var(--color-cyan)]">
                {fmtTokensCompact(b.input_tokens)}
              </td>
              <td className="px-2 py-1.5 text-right text-[var(--color-magenta)]">
                {fmtTokensCompact(b.output_tokens)}
              </td>
              <td className="px-2 py-1.5 text-right text-[var(--color-dim)]">
                {fmtTokensCompact(b.cache_read_tokens + b.cache_create_tokens)}
              </td>
              <td
                className="px-2 py-1.5 text-right font-semibold"
                style={{ color: "var(--color-lime)" }}
              >
                {fmtUsd(b.cost_usd)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** SVG stacked bar sparkline — no external deps */
interface SparklineProps {
  buckets: UsageBucket[];
  mode: "tokens" | "cost";
  loading: boolean;
}

interface TooltipState {
  x: number;
  y: number;
  bucket: UsageBucket;
}

function Sparkline({ buckets, mode, loading }: SparklineProps) {
  const svgRef = React.useRef<SVGSVGElement>(null);
  const [tooltip, setTooltip] = React.useState<TooltipState | null>(null);

  const WIDTH = 800;
  const HEIGHT = 100;
  const PAD_LEFT = 40;
  const PAD_BOTTOM = 20;
  const PAD_TOP = 8;
  const PAD_RIGHT = 8;
  const plotW = WIDTH - PAD_LEFT - PAD_RIGHT;
  const plotH = HEIGHT - PAD_BOTTOM - PAD_TOP;

  const data = React.useMemo(() => {
    if (buckets.length === 0) return [];
    return buckets.map((b) => ({
      bucket: b,
      input: mode === "tokens" ? b.input_tokens : b.cost_usd * 0.3,
      output: mode === "tokens" ? b.output_tokens : b.cost_usd * 0.5,
      cache:
        mode === "tokens"
          ? b.cache_read_tokens + b.cache_create_tokens
          : b.cost_usd * 0.2,
      total: mode === "tokens"
        ? b.input_tokens + b.output_tokens + b.cache_read_tokens + b.cache_create_tokens
        : b.cost_usd,
    }));
  }, [buckets, mode]);

  const maxVal = React.useMemo(
    () => Math.max(...data.map((d) => d.total), 1),
    [data],
  );

  if (loading) {
    return (
      <div className="flex items-end gap-1 px-2" style={{ height: HEIGHT }}>
        {Array.from({ length: 30 }, (_, i) => (
          <Skeleton
            key={i}
            style={{ flex: 1, height: `${20 + Math.random() * 60}%` }}
          />
        ))}
      </div>
    );
  }

  if (data.length === 0) {
    return (
      <div
        className="flex items-center justify-center font-mono text-[11px] text-[var(--color-dim)]"
        style={{ height: HEIGHT }}
      >
        ▸ sin datos
      </div>
    );
  }

  const n = data.length;
  const barW = Math.max(4, (plotW / n) * 0.75);
  const gap = plotW / n;

  function toY(v: number): number {
    return PAD_TOP + plotH - (v / maxVal) * plotH;
  }

  function formatYLabel(v: number): string {
    return mode === "tokens" ? fmtTokensCompact(v) : `$${v.toFixed(2)}`;
  }

  const yTicks = [0, 0.25, 0.5, 0.75, 1].map((t) => ({
    val: maxVal * t,
    y: toY(maxVal * t),
  }));

  // Format x-axis label: show day/month from the key (e.g. "2026-04-30" → "30/4")
  function fmtXLabel(key: string): string {
    const parts = key.split("-");
    if (parts.length !== 3) return key;
    return `${parts[2]}/${parts[1]}`;
  }

  // Show x-labels sparsely to avoid overlap
  const xLabelEvery = n > 14 ? Math.ceil(n / 7) : 1;

  function handleMouseMove(e: React.MouseEvent<SVGSVGElement>) {
    if (!svgRef.current) return;
    const rect = svgRef.current.getBoundingClientRect();
    const scaleX = WIDTH / rect.width;
    const mx = (e.clientX - rect.left) * scaleX;
    const relX = mx - PAD_LEFT;
    const idx = Math.min(n - 1, Math.max(0, Math.floor(relX / gap)));
    if (idx >= 0 && idx < data.length) {
      setTooltip({ x: PAD_LEFT + idx * gap + gap / 2, y: toY(data[idx].total), bucket: data[idx].bucket });
    }
  }

  function handleMouseLeave() {
    setTooltip(null);
  }

  return (
    <div className="relative w-full" style={{ paddingBottom: 0 }}>
      <svg
        ref={svgRef}
        viewBox={`0 0 ${WIDTH} ${HEIGHT}`}
        width="100%"
        preserveAspectRatio="none"
        onMouseMove={handleMouseMove}
        onMouseLeave={handleMouseLeave}
        style={{ display: "block", cursor: "crosshair" }}
      >
        {/* y grid lines */}
        {yTicks.map(({ val, y }) => (
          <g key={val}>
            <line
              x1={PAD_LEFT}
              x2={WIDTH - PAD_RIGHT}
              y1={y}
              y2={y}
              stroke="rgba(255,255,255,0.05)"
              strokeWidth={0.5}
            />
            <text
              x={PAD_LEFT - 4}
              y={y + 3}
              textAnchor="end"
              fontSize={7}
              fill="rgba(255,255,255,0.3)"
            >
              {formatYLabel(val)}
            </text>
          </g>
        ))}

        {/* bars */}
        {data.map((d, i) => {
          const bx = PAD_LEFT + i * gap + gap / 2 - barW / 2;
          const totalH = ((d.total / maxVal) * plotH) || 0;
          const inputH = ((d.input / maxVal) * plotH) || 0;
          const outputH = ((d.output / maxVal) * plotH) || 0;
          const cacheH = totalH - inputH - outputH;

          // stack from bottom: cache, output, input
          let stackY = PAD_TOP + plotH;

          const cacheY = stackY - cacheH;
          stackY = cacheY;
          const outputY = stackY - outputH;
          stackY = outputY;
          const inputY = stackY - inputH;

          return (
            <g key={d.bucket.key}>
              {cacheH > 0 && (
                <rect
                  x={bx}
                  y={cacheY}
                  width={barW}
                  height={cacheH}
                  fill="rgba(255,255,255,0.18)"
                />
              )}
              {outputH > 0 && (
                <rect
                  x={bx}
                  y={outputY}
                  width={barW}
                  height={outputH}
                  fill="#ff4ed6aa"
                />
              )}
              {inputH > 0 && (
                <rect
                  x={bx}
                  y={inputY}
                  width={barW}
                  height={inputH}
                  fill="#5ef0ffcc"
                />
              )}
              {/* x-axis label */}
              {i % xLabelEvery === 0 && (
                <text
                  x={PAD_LEFT + i * gap + gap / 2}
                  y={HEIGHT - 4}
                  textAnchor="middle"
                  fontSize={7}
                  fill="rgba(255,255,255,0.3)"
                >
                  {fmtXLabel(d.bucket.key)}
                </text>
              )}
            </g>
          );
        })}

        {/* hover line */}
        {tooltip && (
          <line
            x1={tooltip.x}
            x2={tooltip.x}
            y1={PAD_TOP}
            y2={PAD_TOP + plotH}
            stroke="rgba(255,255,255,0.25)"
            strokeWidth={1}
            strokeDasharray="3 2"
          />
        )}
      </svg>

      {/* tooltip */}
      {tooltip && (
        <div
          className="pointer-events-none absolute top-0 z-10 clip-hud-sm px-2 py-1.5 font-mono text-[9px]"
          style={{
            left: `${(tooltip.x / WIDTH) * 100}%`,
            transform: tooltip.x > WIDTH * 0.7 ? "translateX(-105%)" : "translateX(8px)",
            background: "rgba(6,8,20,0.92)",
            border: "1px solid rgba(94,240,255,0.35)",
            color: "var(--color-fg)",
          }}
        >
          <div className="text-[var(--color-cyan)] mb-1">{tooltip.bucket.key}</div>
          {mode === "tokens" ? (
            <>
              <div>input: {fmtTokensCompact(tooltip.bucket.input_tokens)}</div>
              <div>output: {fmtTokensCompact(tooltip.bucket.output_tokens)}</div>
              <div>
                cache:{" "}
                {fmtTokensCompact(
                  tooltip.bucket.cache_read_tokens + tooltip.bucket.cache_create_tokens,
                )}
              </div>
            </>
          ) : (
            <div>costo: {fmtUsd(tooltip.bucket.cost_usd)}</div>
          )}
          <div className="mt-1 text-[var(--color-dim)]">{tooltip.bucket.events} eventos</div>
        </div>
      )}

      {/* legend */}
      <div className="flex items-center gap-4 mt-2 font-mono text-[9px] text-[var(--color-dim)]">
        <span className="flex items-center gap-1">
          <span className="w-3 h-2 inline-block" style={{ background: "#5ef0ffcc" }} />
          input
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-2 inline-block" style={{ background: "#ff4ed6aa" }} />
          output
        </span>
        <span className="flex items-center gap-1">
          <span className="w-3 h-2 inline-block" style={{ background: "rgba(255,255,255,0.18)" }} />
          cache
        </span>
      </div>
    </div>
  );
}

// ─── main page ────────────────────────────────────────────────────────────────

const REALTIME_POLL_MS = 60_000;

interface PeriodData {
  totals: { cost: number; tokens: number } | null;
}

export function Usage() {
  // ── historical: 24h, 7d, 30d summaries
  const [data24h, setData24h] = React.useState<PeriodData>({ totals: null });
  const [data7d, setData7d] = React.useState<PeriodData>({ totals: null });
  const [data30d, setData30d] = React.useState<PeriodData>({ totals: null });
  const [prev7d, setPrev7d] = React.useState<PeriodData>({ totals: null });
  const [prev30d, setPrev30d] = React.useState<PeriodData>({ totals: null });

  // ── breakdown by model
  const [modelBuckets, setModelBuckets] = React.useState<UsageBucket[]>([]);
  const [modelLoading, setModelLoading] = React.useState(true);

  // ── sparkline by day
  const [dayBuckets, setDayBuckets] = React.useState<UsageBucket[]>([]);
  const [dayLoading, setDayLoading] = React.useState(true);
  const [sparkMode, setSparkMode] = React.useState<"tokens" | "cost">("tokens");

  // ── realtime
  const [realtime, setRealtime] = React.useState<RealtimeResponse | null>(null);
  const [realtimeLoading, setRealtimeLoading] = React.useState(true);
  const [realtimeUnavailable, setRealtimeUnavailable] = React.useState(false);
  const [realtimeUpdatedAt, setRealtimeUpdatedAt] = React.useState<number | null>(null);

  // ── error
  const [histError, setHistError] = React.useState<string | null>(null);

  // ── summary cards loading
  const summaryLoading = data30d.totals === null;

  // ─── historical fetch ───────────────────────────────────────────────────────

  const fetchHistorical = React.useCallback(async () => {
    try {
      const now = Math.floor(Date.now() / 1000);
      const [r24h, r7d, r30d, rPrev7d, rPrev30d] = await Promise.all([
        api.getUsage({ since: now - 86_400 }),
        api.getUsage({ since: "7d" }),
        api.getUsage({ since: "30d" }),
        // previous 7d period: 14d ago → 7d ago
        api.getUsage({ since: now - 14 * 86_400, until: now - 7 * 86_400 }),
        // previous 30d period: 60d ago → 30d ago
        api.getUsage({ since: now - 60 * 86_400, until: now - 30 * 86_400 }),
      ]);
      setData24h({ totals: { cost: r24h.totals.cost_usd, tokens: r24h.totals.input_tokens + r24h.totals.output_tokens } });
      setData7d({ totals: { cost: r7d.totals.cost_usd, tokens: r7d.totals.input_tokens + r7d.totals.output_tokens } });
      setData30d({ totals: { cost: r30d.totals.cost_usd, tokens: r30d.totals.input_tokens + r30d.totals.output_tokens } });
      setPrev7d({ totals: { cost: rPrev7d.totals.cost_usd, tokens: rPrev7d.totals.input_tokens + rPrev7d.totals.output_tokens } });
      setPrev30d({ totals: { cost: rPrev30d.totals.cost_usd, tokens: rPrev30d.totals.input_tokens + rPrev30d.totals.output_tokens } });
      setHistError(null);
    } catch (err) {
      setHistError(err instanceof Error ? err.message : "error cargando histórico");
    }
  }, []);

  const fetchModelBreakdown = React.useCallback(async () => {
    setModelLoading(true);
    try {
      const r = await api.getUsage({ group_by: "model", since: "30d" });
      setModelBuckets(r.buckets ?? []);
    } catch {
      setModelBuckets([]);
    } finally {
      setModelLoading(false);
    }
  }, []);

  const fetchDaySparkline = React.useCallback(async () => {
    setDayLoading(true);
    try {
      const r = await api.getUsage({ group_by: "day", since: "30d" });
      setDayBuckets(r.buckets ?? []);
    } catch {
      setDayBuckets([]);
    } finally {
      setDayLoading(false);
    }
  }, []);

  // ─── realtime fetch ─────────────────────────────────────────────────────────

  const fetchRealtime = React.useCallback(async () => {
    try {
      const r = await api.getUsageRealtime();
      setRealtime(r);
      setRealtimeUnavailable(false);
      setRealtimeUpdatedAt(Math.floor(Date.now() / 1000));
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setRealtimeUnavailable(true);
      }
      // on any error: keep last-known data, mark unavailable gracefully
    } finally {
      setRealtimeLoading(false);
    }
  }, []);

  // ─── effects ────────────────────────────────────────────────────────────────

  React.useEffect(() => {
    void fetchHistorical();
    void fetchModelBreakdown();
    void fetchDaySparkline();
  }, [fetchHistorical, fetchModelBreakdown, fetchDaySparkline]);

  React.useEffect(() => {
    void fetchRealtime();
    const id = window.setInterval(fetchRealtime, REALTIME_POLL_MS);
    return () => window.clearInterval(id);
  }, [fetchRealtime]);

  // ─── derived ────────────────────────────────────────────────────────────────

  const total30dTokens =
    data30d.totals
      ? modelBuckets.reduce((s, b) => s + b.input_tokens + b.output_tokens, 0)
      : 0;

  const isEmpty = data30d.totals !== null && data30d.totals.tokens === 0 && total30dTokens === 0 && modelBuckets.length === 0;

  // ─── render ─────────────────────────────────────────────────────────────────

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Usage" }]}
        status={{ label: histError ? "ERROR" : "NOMINAL", tone: histError ? "danger" : "ok" }}
        right={
          <div className="flex items-center gap-3">
            {realtimeUpdatedAt && (
              <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight hidden sm:inline">
                rt: {fmtStamp(realtimeUpdatedAt)}
              </span>
            )}
            <button
              type="button"
              onClick={() => void fetchRealtime()}
              className="flex items-center gap-1.5 px-2 py-1 clip-tag font-mono text-[9px] uppercase tracking-hud-tight cursor-pointer transition-opacity hover:opacity-80"
              style={{
                color: "var(--color-cyan)",
                border: "1px solid rgba(94,240,255,0.35)",
                background: "rgba(94,240,255,0.06)",
              }}
            >
              <RefreshCw size={10} strokeWidth={1.8} />
              refresh realtime
            </button>
          </div>
        }
      />

      <div className="flex-1 min-h-0 p-2 overflow-y-auto sm:p-4 flex flex-col gap-4">

        {/* ── historical error banner ── */}
        {histError && (
          <div
            className="px-3 py-2 font-mono text-[10px] clip-hud-sm"
            style={{
              background: "rgba(255,92,122,0.08)",
              border: "1px solid rgba(255,92,122,0.45)",
              color: "var(--color-danger)",
            }}
          >
            ✗ {histError}
          </div>
        )}

        {/* ── empty state ── */}
        {isEmpty && (
          <div className="flex-1 flex items-center justify-center">
            <HudPanel accent="cyan" className="max-w-sm w-full">
              <div className="flex flex-col items-center gap-3 py-6 px-4 text-center">
                <TrendingUp size={28} style={{ color: "var(--color-cyan)" }} strokeWidth={1.4} />
                <div className="font-display text-[13px] font-semibold tracking-hud text-[var(--color-fg)]">
                  SIN DATOS AÚN
                </div>
                <div className="font-mono text-[11px] text-[var(--color-dim)]">
                  el worker aún no completó el primer scan, volvé en 5 min
                </div>
              </div>
            </HudPanel>
          </div>
        )}

        {/* ── top band: 4 stat cards ── */}
        {!isEmpty && (
          <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
            <StatCard
              label="Cost · 24h"
              value={data24h.totals ? fmtUsd(data24h.totals.cost) : "—"}
              delta={
                data24h.totals && data7d.totals
                  ? fmtDelta(data24h.totals.cost, data7d.totals.cost / 7, true)
                  : undefined
              }
              deltaPositiveBad
              accent="var(--color-lime)"
              loading={summaryLoading}
            />
            <StatCard
              label="Cost · 7d"
              value={data7d.totals ? fmtUsd(data7d.totals.cost) : "—"}
              delta={
                data7d.totals && prev7d.totals
                  ? fmtDelta(data7d.totals.cost, prev7d.totals.cost, true)
                  : undefined
              }
              deltaPositiveBad
              accent="var(--color-cyan)"
              loading={summaryLoading}
            />
            <StatCard
              label="Cost · 30d"
              value={data30d.totals ? fmtUsd(data30d.totals.cost) : "—"}
              delta={
                data30d.totals && prev30d.totals
                  ? fmtDelta(data30d.totals.cost, prev30d.totals.cost, true)
                  : undefined
              }
              deltaPositiveBad
              accent="var(--color-magenta)"
              loading={summaryLoading}
            />
            <StatCard
              label="Tokens · 30d"
              value={
                total30dTokens > 0
                  ? fmtTokensCompact(total30dTokens)
                  : data30d.totals
                  ? fmtTokensCompact(data30d.totals.tokens)
                  : "—"
              }
              accent="var(--color-orange)"
              loading={summaryLoading}
            />
          </div>
        )}

        {/* ── middle: realtime (left) + model table (right) ── */}
        {!isEmpty && (
          <div className="grid grid-cols-1 gap-4 xl:grid-cols-5">

            {/* realtime cards */}
            <div className="xl:col-span-2">
              <HudPanel title="realtime" sub="session · weekly · credits" accent="cyan">
                {realtimeUnavailable ? (
                  <div
                    className="mb-3 px-3 py-2 clip-hud-sm font-mono text-[10px]"
                    style={{
                      color: "var(--color-warn)",
                      border: "1px solid rgba(255,184,108,0.35)",
                      background: "rgba(255,184,108,0.07)",
                    }}
                  >
                    ▸ realtime no disponible — pedilo OAuth login
                  </div>
                ) : null}
                <div className="flex flex-col gap-3">
                  <ProviderCard
                    name="CLAUDE"
                    provider={realtime?.claude}
                    loading={realtimeLoading && !realtimeUnavailable}
                  />
                  <ProviderCard
                    name="CODEX"
                    provider={realtime?.codex}
                    loading={realtimeLoading && !realtimeUnavailable}
                  />
                </div>
              </HudPanel>
            </div>

            {/* model breakdown */}
            <div className="xl:col-span-3">
              <HudPanel
                title="breakdown por modelo"
                sub="30d · sort por costo"
                accent="magenta"
                className="h-full"
              >
                <ModelTable buckets={modelBuckets} loading={modelLoading} />
              </HudPanel>
            </div>
          </div>
        )}

        {/* ── bottom: sparkline 30d ── */}
        {!isEmpty && (
          <HudPanel
            title="distribución 30 días"
            sub="input · output · cache"
            accent="orange"
          >
            <div className="flex items-center justify-end mb-3 gap-2">
              {(["tokens", "cost"] as const).map((m) => (
                <button
                  key={m}
                  type="button"
                  onClick={() => setSparkMode(m)}
                  className="px-2 py-1 clip-tag font-mono text-[9px] uppercase tracking-hud-tight cursor-pointer transition-opacity"
                  style={{
                    color: sparkMode === m ? "var(--color-orange)" : "var(--color-dim)",
                    border: `1px solid ${sparkMode === m ? "rgba(255,159,67,0.55)" : "var(--color-line)"}`,
                    background: sparkMode === m ? "rgba(255,159,67,0.10)" : "transparent",
                  }}
                >
                  {m}
                </button>
              ))}
            </div>
            <Sparkline buckets={dayBuckets} mode={sparkMode} loading={dayLoading} />
          </HudPanel>
        )}

        {/* ── footer ── */}
        <div className="flex items-center justify-between font-mono text-[9px] text-[var(--color-dim)] pb-2">
          <span>histórico: worker scan cada ~5 min</span>
          <span>
            auto-refresh realtime cada {REALTIME_POLL_MS / 1000}s
            {realtimeUpdatedAt && ` · último: ${fmtStamp(realtimeUpdatedAt)}`}
          </span>
        </div>

      </div>
    </div>
  );
}
