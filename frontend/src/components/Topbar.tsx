import * as React from "react";
import { ChevronRight } from "lucide-react";
import { Link } from "react-router-dom";

interface TopbarProps {
  breadcrumb: Array<{ label: string; href?: string }>;
  status?: { label: string; tone?: "ok" | "warn" | "danger" };
  right?: React.ReactNode;
}

const TONE: Record<NonNullable<NonNullable<TopbarProps["status"]>["tone"]>, string> = {
  ok: "var(--color-lime)",
  warn: "var(--color-warn)",
  danger: "var(--color-danger)",
};

export function Topbar({ breadcrumb, status, right }: TopbarProps) {
  const tone = status?.tone ? TONE[status.tone] : "var(--color-lime)";
  return (
    <header
      className="flex items-center justify-between px-6 py-3 border-b border-[var(--color-line)] relative z-10"
      style={{
        background:
          "linear-gradient(180deg, rgba(12,18,40,0.9), rgba(6,8,20,0.4))",
      }}
    >
      <nav className="flex items-center gap-2 font-mono text-[11px] tracking-hud-tight">
        {breadcrumb.map((crumb, i) => {
          const isLast = i === breadcrumb.length - 1;
          const className = isLast
            ? "text-[var(--color-fg)] font-display uppercase tracking-hud font-semibold"
            : "text-[var(--color-dim)] uppercase";
          return (
            <React.Fragment key={`${crumb.label}-${i}`}>
              {i > 0 && (
                <ChevronRight size={11} className="text-[var(--color-dim)]" />
              )}
              {!isLast && crumb.href ? (
                <Link to={crumb.href} className={`${className} hover:text-[var(--color-cyan)] transition-colors`}>
                  {crumb.label}
                </Link>
              ) : (
                <span className={className}>{crumb.label}</span>
              )}
            </React.Fragment>
          );
        })}
      </nav>

      <div className="flex items-center gap-4">
        {right}
        {status && (
          <div
            className="px-3 py-1 clip-tag font-mono text-[10px] tracking-hud font-semibold flex items-center gap-2"
            style={{
              border: `1px solid ${tone}`,
              background: `linear-gradient(90deg, ${tone}25, transparent)`,
              color: tone,
            }}
          >
            <span
              className="w-1.5 h-1.5 rounded-full"
              style={{
                background: tone,
                boxShadow: `0 0 6px ${tone}`,
              }}
            />
            {status.label}
          </div>
        )}
      </div>
    </header>
  );
}
