import * as React from "react";
import {
  Plus,
  Save,
  Sparkles,
  Wand2,
  Trash2,
  Copy,
  Check,
} from "lucide-react";

import { HudPanel } from "@/components/HudPanel";
import { MermaidBlock } from "@/components/MermaidBlock";
import { Topbar } from "@/components/Topbar";
import {
  api,
  type Diagram,
  type DiagramPayload,
  type DiagramType,
} from "@/lib/api";

const DEFAULT_MERMAID =
  "flowchart LR\n  User[Usuario] --> UI[AgentHub UI]\n  UI --> API[Go API]\n  API --> DB[(SQLite)]";

function rel(ts?: number): string {
  if (!ts) return "—";
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts);
  if (s < 60) return "ahora";
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export function Diagrams() {
  const [diagrams, setDiagrams] = React.useState<Diagram[]>([]);
  const [selectedId, setSelectedId] = React.useState<number | null>(null);
  const [title, setTitle] = React.useState("Nuevo diagrama");
  const [prompt, setPrompt] = React.useState(
    "diagrama de arquitectura de AgentHub",
  );
  const [type, setType] = React.useState<DiagramType | "auto">("auto");
  const [mermaid, setMermaid] = React.useState<string>(DEFAULT_MERMAID);
  /** what's actually rendered. Decoupled from the editor so the user can edit
   * without re-rendering on every keystroke. */
  const [rendered, setRendered] = React.useState<string>(DEFAULT_MERMAID);
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [copied, setCopied] = React.useState(false);

  const refresh = React.useCallback(async () => {
    try {
      setDiagrams(await api.listDiagrams());
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error cargando diagramas");
    }
  }, []);

  React.useEffect(() => {
    void refresh();
  }, [refresh]);

  function newDiagram() {
    setSelectedId(null);
    setTitle("Nuevo diagrama");
    setPrompt("diagrama de arquitectura de AgentHub");
    setMermaid(DEFAULT_MERMAID);
    setRendered(DEFAULT_MERMAID);
    setError(null);
  }

  function loadDiagram(d: Diagram) {
    setSelectedId(d.id);
    setTitle(d.title);
    setPrompt(d.prompt ?? "");
    const mer = d.mermaid_source || d.mermaid || "";
    setMermaid(mer || DEFAULT_MERMAID);
    setRendered(mer || DEFAULT_MERMAID);
    setError(null);
  }

  async function generate() {
    const p = prompt.trim();
    if (!p) {
      setError("escribí un prompt para generar");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      const out = await api.generateDiagram({
        prompt: p,
        type: type === "auto" ? undefined : type,
      });
      const mer = out.mermaid || "";
      if (!mer) {
        setError("la IA no devolvió mermaid");
      } else {
        setMermaid(mer);
        setRendered(mer);
        if (out.title) setTitle(out.title);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "error generando");
    } finally {
      setBusy(false);
    }
  }

  function applyEdited() {
    setRendered(mermaid);
  }

  async function save() {
    const cleanTitle = title.trim();
    if (!cleanTitle) {
      setError("title requerido");
      return;
    }
    const payload: DiagramPayload = {
      title: cleanTitle,
      prompt: prompt.trim(),
      mermaid,
      // Persist a stub Excalidraw scene for compat — we no longer render it,
      // but the API still requires the column to be non-null.
      excalidraw_json: '{"elements":[],"appState":{}}',
    };
    setBusy(true);
    try {
      const saved = selectedId
        ? await api.updateDiagram(selectedId, payload)
        : await api.createDiagram(payload);
      setSelectedId(saved.id);
      await refresh();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error guardando diagrama");
    } finally {
      setBusy(false);
    }
  }

  async function remove(id: number) {
    if (!window.confirm("¿Borrar este diagrama?")) return;
    setBusy(true);
    try {
      await api.deleteDiagram(id);
      if (selectedId === id) newDiagram();
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : "error borrando");
    } finally {
      setBusy(false);
    }
  }

  async function copyMermaid() {
    try {
      await navigator.clipboard.writeText(mermaid);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      // noop
    }
  }

  return (
    <div className="flex flex-col h-full min-h-0">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Diagramas" }]}
        status={
          error
            ? { label: "ERROR", tone: "danger" }
            : { label: `${diagrams.length} GUARDADOS`, tone: "ok" }
        }
      />

      <div className="flex-1 min-h-0 grid grid-cols-[260px_1fr] gap-3 p-3">
        {/* Left: list */}
        <div className="flex flex-col min-h-0 gap-2">
          <button
            type="button"
            onClick={newDiagram}
            className="w-full px-3 py-2 clip-tag font-mono text-[11px] uppercase tracking-hud-tight border cursor-pointer transition-colors flex items-center gap-2"
            style={{
              color: "var(--color-cyan)",
              borderColor: "var(--color-cyan)",
              background: "rgba(94,240,255,0.06)",
            }}
          >
            <Plus size={12} strokeWidth={1.8} /> nuevo diagrama
          </button>

          <div
            className="flex-1 min-h-0 overflow-y-auto clip-hud-sm border"
            style={{ borderColor: "var(--color-line)" }}
          >
            {diagrams.length === 0 && (
              <div className="px-3 py-4 font-mono text-[10px] text-[var(--color-dim)] italic">
                ▸ aún no hay diagramas guardados.
              </div>
            )}
            {diagrams.map((d) => {
              const active = d.id === selectedId;
              return (
                <div
                  key={d.id}
                  className={`group flex items-center gap-1 px-2 py-2 cursor-pointer text-left border-b transition-colors ${
                    active
                      ? "bg-[rgba(94,240,255,0.10)]"
                      : "hover:bg-[rgba(94,240,255,0.04)]"
                  }`}
                  style={{ borderColor: "var(--color-line)" }}
                  onClick={() => loadDiagram(d)}
                >
                  <div className="flex-1 min-w-0">
                    <div className="font-mono text-[11px] truncate text-[var(--color-fg)]">
                      {d.title}
                    </div>
                    <div className="font-mono text-[9px] text-[var(--color-dim)] tracking-hud-tight">
                      #{d.id} · {rel(d.updated_at)}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={(e) => {
                      e.stopPropagation();
                      void remove(d.id);
                    }}
                    className="opacity-0 group-hover:opacity-100 text-[var(--color-dim)] hover:text-[var(--color-danger)] cursor-pointer p-1 transition-all"
                    title="borrar"
                  >
                    <Trash2 size={11} strokeWidth={1.8} />
                  </button>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right: canvas + prompt + editor */}
        <div className="flex flex-col min-h-0 gap-3">
          <HudPanel title="diagrama" accent="cyan" className="flex-1 min-h-0">
            <div className="flex-1 min-h-0 px-2 pb-2">
              <MermaidBlock content={rendered} size="fill" />
            </div>
          </HudPanel>

          {/* Prompt + actions */}
          <div
            className="px-3 py-2 clip-hud-sm border space-y-2"
            style={{
              borderColor: "var(--color-line)",
              background: "rgba(10,15,36,0.55)",
            }}
          >
            <div className="flex items-center gap-2 flex-wrap">
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="título del diagrama"
                className="font-mono text-[11.5px] px-2 py-1 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)] flex-1 min-w-[160px]"
                style={{ borderColor: "var(--color-line)" }}
              />
              <select
                value={type}
                onChange={(e) => setType(e.target.value as DiagramType | "auto")}
                className="font-mono text-[10px] uppercase tracking-hud-tight px-2 py-1 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)]"
                style={{ borderColor: "var(--color-line)" }}
              >
                <option value="auto">auto</option>
                <option value="flowchart">flowchart</option>
                <option value="sequence">sequence</option>
                <option value="c4">c4</option>
                <option value="erd">erd</option>
                <option value="mindmap">mindmap</option>
              </select>
              <button
                type="button"
                onClick={() => void save()}
                disabled={busy}
                className="px-3 py-1.5 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors disabled:opacity-60"
                style={{
                  color: "var(--color-lime)",
                  borderColor: "var(--color-lime)",
                  background: "rgba(163,255,78,0.06)",
                }}
              >
                <Save size={11} strokeWidth={1.8} className="inline mr-1" />
                {selectedId ? "guardar cambios" : "guardar"}
              </button>
            </div>

            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              placeholder="describí el diagrama (ej. 'arquitectura de mi backend Go')"
              rows={2}
              className="w-full font-mono text-[11.5px] px-2 py-1.5 clip-tag border bg-[rgba(255,255,255,0.02)] text-[var(--color-fg)] resize-y"
              style={{ borderColor: "var(--color-line)" }}
            />

            <div className="flex items-center gap-2 flex-wrap">
              <button
                type="button"
                onClick={() => void generate()}
                disabled={busy}
                className="px-3 py-1.5 clip-tag font-mono text-[10px] uppercase tracking-hud-tight font-semibold cursor-pointer transition-all hover:scale-[1.02] disabled:opacity-60"
                style={{
                  color: "var(--color-bg)",
                  background: "var(--color-cyan)",
                  boxShadow: "0 0 8px rgba(94,240,255,0.50)",
                }}
              >
                <Wand2 size={11} strokeWidth={1.8} className="inline mr-1" />
                {busy ? "generando…" : "generar con IA"}
              </button>
              <button
                type="button"
                onClick={applyEdited}
                disabled={busy}
                className="px-3 py-1.5 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border cursor-pointer transition-colors disabled:opacity-60"
                style={{
                  color: "var(--color-magenta)",
                  borderColor: "var(--color-magenta)",
                  background: "rgba(255,78,214,0.06)",
                }}
              >
                <Sparkles size={11} strokeWidth={1.8} className="inline mr-1" />
                aplicar mermaid editado
              </button>
              <button
                type="button"
                onClick={() => void copyMermaid()}
                className="px-3 py-1.5 clip-tag font-mono text-[10px] uppercase tracking-hud-tight border text-[var(--color-dim)] hover:text-[var(--color-fg)] cursor-pointer transition-colors"
                style={{ borderColor: "var(--color-line)" }}
              >
                {copied ? (
                  <>
                    <Check size={11} strokeWidth={1.8} className="inline mr-1" />
                    copiado
                  </>
                ) : (
                  <>
                    <Copy size={11} strokeWidth={1.8} className="inline mr-1" />
                    copiar mermaid
                  </>
                )}
              </button>
              {error && (
                <span className="font-mono text-[10px] text-[var(--color-danger)]">
                  ⚠ {error}
                </span>
              )}
            </div>

            <details className="group">
              <summary className="cursor-pointer font-mono text-[10px] uppercase tracking-hud-tight text-[var(--color-dim)] hover:text-[var(--color-fg)]">
                ▸ ver / editar mermaid source
              </summary>
              <textarea
                value={mermaid}
                onChange={(e) => setMermaid(e.target.value)}
                rows={10}
                spellCheck={false}
                className="w-full mt-2 font-mono text-[11px] px-2 py-1.5 clip-tag border bg-[rgba(0,0,0,0.50)] text-[var(--color-fg)] resize-y"
                style={{ borderColor: "var(--color-line)" }}
              />
            </details>
          </div>
        </div>
      </div>
    </div>
  );
}
