import * as React from "react";

interface GaugeProps {
  value: number;
  max?: number;
  label: string;
  unit?: string;
  /** Pixel size of the SVG box. Default 130. */
  size?: number;
  /** Stroke color of the filled arc. Use a CSS var like var(--color-cyan). */
  color?: string;
  /** Optional sub-label rendered below the value. */
  sub?: string;
}

/**
 * Radial 270° gauge with glow and tick marks.
 * Mirrors hud.jsx <Gauge/> — pure SVG, no deps.
 */
export function Gauge({
  value,
  max = 100,
  label,
  unit = "%",
  size = 130,
  color = "var(--color-cyan)",
  sub,
}: GaugeProps) {
  const pct = Math.max(0, Math.min(1, value / max));
  const R = size / 2 - 10;
  const cx = size / 2;
  const cy = size / 2;
  const C = Math.PI * R * 1.5; // 270deg arc length
  const dash = C * pct;
  const filterId = React.useId().replace(/:/g, "");

  // tick marks: 10 of them, one every 30deg starting at 135deg
  const ticks = Array.from({ length: 10 }, (_, i) => {
    const angle = ((135 + i * 30) * Math.PI) / 180;
    const r1 = R + 6;
    const r2 = R + 11;
    const active = (i * (max / 10)) <= value;
    return {
      x1: cx + Math.cos(angle) * r1,
      y1: cy + Math.sin(angle) * r1,
      x2: cx + Math.cos(angle) * r2,
      y2: cy + Math.sin(angle) * r2,
      active,
    };
  });

  return (
    <div
      className="relative"
      style={{ width: size, height: size }}
    >
      <svg width={size} height={size} style={{ overflow: "visible" }}>
        <defs>
          <filter id={`glow-${filterId}`}>
            <feGaussianBlur stdDeviation="2.5" result="b" />
            <feMerge>
              <feMergeNode in="b" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>
        {/* track */}
        <circle
          cx={cx}
          cy={cy}
          r={R}
          stroke="var(--color-line)"
          strokeWidth={6}
          fill="none"
          strokeDasharray={`${C} 9999`}
          transform={`rotate(135 ${cx} ${cy})`}
        />
        {/* fill */}
        <circle
          cx={cx}
          cy={cy}
          r={R}
          stroke={color}
          strokeWidth={6}
          fill="none"
          strokeLinecap="round"
          strokeDasharray={`${dash} 9999`}
          transform={`rotate(135 ${cx} ${cy})`}
          filter={`url(#glow-${filterId})`}
          style={{ transition: "stroke-dasharray 600ms ease" }}
        />
        {/* ticks */}
        {ticks.map((t, i) => (
          <line
            key={i}
            x1={t.x1}
            y1={t.y1}
            x2={t.x2}
            y2={t.y2}
            stroke={t.active ? color : "var(--color-line)"}
            strokeWidth={1.5}
          />
        ))}
      </svg>

      <div
        className="absolute inset-0 flex flex-col items-center justify-center"
      >
        <div
          className="font-display font-bold leading-none text-[var(--color-fg)]"
          style={{ fontSize: size * 0.28 }}
        >
          {Math.round(value)}
          <span
            className="text-[var(--color-dim)] ml-0.5"
            style={{ fontSize: size * 0.14 }}
          >
            {unit}
          </span>
        </div>
        <div
          className="font-mono tracking-hud mt-1"
          style={{ fontSize: 10, color }}
        >
          {label}
        </div>
        {sub && (
          <div
            className="font-mono mt-0.5 text-[var(--color-dim)]"
            style={{ fontSize: 9 }}
          >
            {sub}
          </div>
        )}
      </div>
    </div>
  );
}
