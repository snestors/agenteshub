import * as React from "react";
import { cn } from "@/lib/utils";

interface HudPanelProps {
  title?: string;
  sub?: string;
  accent?: "cyan" | "magenta" | "lime" | "orange" | "danger";
  children: React.ReactNode;
  className?: string;
  bodyClassName?: string;
}

const ACCENT: Record<NonNullable<HudPanelProps["accent"]>, { color: string; rgb: string }> = {
  cyan: { color: "var(--color-cyan)", rgb: "94, 240, 255" },
  magenta: { color: "var(--color-magenta)", rgb: "255, 78, 214" },
  lime: { color: "var(--color-lime)", rgb: "163, 255, 78" },
  orange: { color: "var(--color-orange)", rgb: "255, 159, 67" },
  danger: { color: "var(--color-danger)", rgb: "255, 92, 122" },
};

/**
 * HudPanel — corner-cut clip-path panel matching hud.jsx.
 * Outer shell does the gradient stroke; inner content sits on bg-2.
 */
export function HudPanel({
  title,
  sub,
  accent = "cyan",
  children,
  className,
  bodyClassName,
}: HudPanelProps) {
  const a = ACCENT[accent];
  return (
    <div
      className={cn("relative clip-hud p-[2px] h-full", className)}
      style={{
        background: `linear-gradient(135deg, rgba(${a.rgb}, 0.32), rgba(${a.rgb}, 0.05) 40%, rgba(${a.rgb}, 0.20))`,
      }}
    >
      <div
        className={cn(
          "relative clip-hud bg-[var(--color-bg-2)] h-full w-full px-4 py-3 flex flex-col min-h-0",
          bodyClassName
        )}
      >
        {title && (
          <div className="flex items-center justify-between mb-2 pb-1.5 border-b border-[var(--color-line)]">
            <div className="flex items-center gap-2">
              <span
                className="w-1.5 h-1.5 rounded-full"
                style={{
                  background: a.color,
                  boxShadow: `0 0 8px ${a.color}`,
                }}
              />
              <span
                className="font-display text-[12px] font-semibold uppercase tracking-hud"
                style={{ color: "var(--color-fg)" }}
              >
                {title}
              </span>
            </div>
            {sub && (
              <span className="font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
                {sub}
              </span>
            )}
          </div>
        )}
        <div className="flex-1 min-h-0 flex flex-col">{children}</div>
      </div>
    </div>
  );
}
