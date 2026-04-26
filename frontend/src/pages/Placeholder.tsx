import { HudPanel } from "@/components/HudPanel";
import { Topbar } from "@/components/Topbar";

interface PlaceholderProps {
  title: string;
  subtitle: string;
  description: string;
  accent?: "cyan" | "magenta" | "lime" | "orange";
}

export function Placeholder({
  title,
  subtitle,
  description,
  accent = "cyan",
}: PlaceholderProps) {
  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: title }]}
        status={{ label: "EN CONSTRUCCIÓN", tone: "warn" }}
      />

      <div className="flex-1 min-h-0 p-4 overflow-hidden">
        <HudPanel title={title} sub={subtitle} accent={accent} className="h-full">
          <div className="flex-1 grid place-items-center">
            <div className="max-w-md text-center px-6 py-10">
              <div className="font-display text-[22px] tracking-hud text-[var(--color-fg)] uppercase mb-3">
                v1+ próximamente
              </div>
              <div className="font-mono text-[12px] leading-relaxed text-[var(--color-dim)]">
                {description}
              </div>

              <div className="mt-6 flex items-center justify-center gap-2 font-mono text-[10px] text-[var(--color-dim)] tracking-hud-tight">
                <span
                  className="w-1.5 h-1.5 rounded-full"
                  style={{
                    background: "var(--color-warn)",
                    boxShadow: "0 0 6px var(--color-warn)",
                  }}
                />
                BACKEND PENDIENTE
              </div>
            </div>
          </div>
        </HudPanel>
      </div>
    </div>
  );
}
