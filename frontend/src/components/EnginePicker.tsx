import * as React from "react";
import {
  api,
  ApiError,
  FALLBACK_ENGINES,
  type EngineDef,
} from "@/lib/api";
import { wsClient } from "@/lib/wsClient";

interface EnginePickerProps {
  /** currently active engine (drives initial selection) */
  currentEngine: string;
  /** currently active model (drives initial selection) */
  currentModel: string;
  /** invoked after a successful POST /api/agent/engine */
  onApplied: (engine: string, model: string) => void;
  /** invoked when the user dismisses the picker (Esc, click outside, Cancel) */
  onClose: () => void;
}

function fmtCtxWindow(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(0)}M ctx`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}K ctx`;
  return `${n} ctx`;
}

/**
 * EnginePicker — floating dropdown over StatusBar's engine badge.
 * Lets the user switch engine + model over the unified /ws RPC channel.
 */
export function EnginePicker({
  currentEngine,
  currentModel,
  onApplied,
  onClose,
}: EnginePickerProps) {
  const [engines, setEngines] = React.useState<EngineDef[]>(FALLBACK_ENGINES);
  const [selectedEngine, setSelectedEngine] = React.useState(currentEngine);
  const [selectedModel, setSelectedModel] = React.useState(currentModel);
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  // Fetch engines list. Fallback silently to FALLBACK_ENGINES on 404 / network.
  React.useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const list = await api.listEngines();
        if (cancelled) return;
        if (list.length > 0) setEngines(list);
      } catch (err) {
        if (cancelled) return;
        if (!(err instanceof ApiError)) {
          // network — keep fallback silently
        }
        // 404 → keep FALLBACK_ENGINES silently
      }
    })();
    return () => {
      cancelled = true;
    };
  }, []);

  // When the engine changes, snap model to first available if current isn't valid.
  React.useEffect(() => {
    const def = engines.find((e) => e.engine === selectedEngine);
    if (!def) return;
    if (!def.models.includes(selectedModel)) {
      setSelectedModel(def.models[0] ?? "");
    }
  }, [selectedEngine, engines]); // eslint-disable-line react-hooks/exhaustive-deps

  // Esc closes the picker.
  React.useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  const currentEngineDef = engines.find((e) => e.engine === selectedEngine);
  const models = currentEngineDef?.models ?? [];
  const ctxWindows = currentEngineDef?.ctx_windows ?? {};

  async function handleApply() {
    if (!selectedEngine || !selectedModel) return;
    setSubmitting(true);
    setError(null);
    try {
      await wsClient.request("set_engine", {
        engine: selectedEngine,
        model: selectedModel,
      });
      onApplied(selectedEngine, selectedModel);
    } catch (err) {
      const msg =
        err instanceof ApiError
          ? `error ${err.status}: ${err.message || "no se pudo aplicar"}`
          : err instanceof Error
          ? err.message
          : "error de red al cambiar engine";
      setError(msg);
      setSubmitting(false);
    }
  }

  return (
    <>
      {/* invisible overlay — click outside dismisses */}
      <div
        className="fixed inset-0 z-40"
        onClick={onClose}
        aria-hidden="true"
      />

      {/* dropdown panel — fixed, not inside StatusBar overflow, so it stays clickable. */}
      <div
        className="fixed left-3 right-3 bottom-14 z-50 max-h-[70vh] overflow-y-auto clip-hud-sm font-mono text-[11px] tracking-hud-tight sm:left-4 sm:right-auto sm:w-[360px]"
        style={{
          background: "rgba(10, 15, 36, 0.95)",
          border: "1px solid rgba(94, 240, 255, 0.55)",
          boxShadow: "0 0 16px rgba(94, 240, 255, 0.35)",
          color: "var(--color-fg)",
          minWidth: 260,
          padding: "10px 12px",
        }}
        role="dialog"
        aria-label="engine picker"
        onClick={(e) => e.stopPropagation()}
      >
        <div
          className="text-[10px] uppercase tracking-hud mb-2"
          style={{ color: "var(--color-cyan)" }}
        >
          engine · model
        </div>

        {/* ENGINE list */}
        <div className="mb-3">
          <div
            className="text-[9px] uppercase tracking-hud mb-1"
            style={{ color: "var(--color-dim)" }}
          >
            engine
          </div>
          <div className="flex flex-col gap-1">
            {engines.map((eng) => {
              const active = eng.engine === selectedEngine;
              return (
                <button
                  key={eng.engine}
                  type="button"
                  onClick={() => setSelectedEngine(eng.engine)}
                  className="flex items-center gap-2 px-2 py-1 text-left clip-tag cursor-pointer"
                  style={{
                    background: active
                      ? "rgba(94, 240, 255, 0.14)"
                      : "transparent",
                    border: active
                      ? "1px solid rgba(94, 240, 255, 0.55)"
                      : "1px solid rgba(120, 255, 220, 0.12)",
                    color: active ? "var(--color-cyan)" : "var(--color-fg)",
                  }}
                >
                  <span aria-hidden="true">{active ? "◉" : "○"}</span>
                  <span>{eng.engine}</span>
                </button>
              );
            })}
          </div>
        </div>

        {/* MODEL list (filtered by selected engine) */}
        <div className="mb-3">
          <div
            className="text-[9px] uppercase tracking-hud mb-1"
            style={{ color: "var(--color-dim)" }}
          >
            model ({selectedEngine})
          </div>
          <div className="flex flex-col gap-1">
            {models.map((m) => {
              const active = m === selectedModel;
              const ctx = ctxWindows[m];
              return (
                <button
                  key={m}
                  type="button"
                  onClick={() => setSelectedModel(m)}
                  className="flex items-center gap-2 px-2 py-1 text-left clip-tag cursor-pointer"
                  style={{
                    background: active
                      ? "rgba(94, 240, 255, 0.14)"
                      : "transparent",
                    border: active
                      ? "1px solid rgba(94, 240, 255, 0.55)"
                      : "1px solid rgba(120, 255, 220, 0.12)",
                    color: active ? "var(--color-cyan)" : "var(--color-fg)",
                  }}
                >
                  <span aria-hidden="true">{active ? "◉" : "○"}</span>
                  <span className="flex-1">{m}</span>
                  {ctx ? (
                    <span style={{ color: "var(--color-dim)" }}>
                      {fmtCtxWindow(ctx)}
                    </span>
                  ) : null}
                </button>
              );
            })}
            {models.length === 0 ? (
              <div style={{ color: "var(--color-dim)" }}>(sin modelos)</div>
            ) : null}
          </div>
        </div>

        {/* error line */}
        {error ? (
          <div
            className="mb-2 px-2 py-1 clip-tag"
            style={{
              background: "rgba(255, 92, 122, 0.12)",
              border: "1px solid rgba(255, 92, 122, 0.55)",
              color: "var(--color-danger)",
            }}
          >
            {error}
          </div>
        ) : null}

        {/* actions */}
        <div className="flex items-center gap-2 justify-end">
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="px-3 py-1 clip-tag cursor-pointer"
            style={{
              background: "transparent",
              border: "1px solid rgba(120, 255, 220, 0.25)",
              color: "var(--color-dim)",
            }}
          >
            Cancelar
          </button>
          <button
            type="button"
            onClick={handleApply}
            disabled={submitting || !selectedEngine || !selectedModel}
            className="px-3 py-1 clip-tag cursor-pointer"
            style={{
              background: "rgba(94, 240, 255, 0.18)",
              border: "1px solid rgba(94, 240, 255, 0.65)",
              color: "var(--color-cyan)",
              opacity: submitting ? 0.6 : 1,
            }}
          >
            {submitting ? "..." : "Aplicar"}
          </button>
        </div>
      </div>
    </>
  );
}
