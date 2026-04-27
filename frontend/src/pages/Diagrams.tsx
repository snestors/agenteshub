import * as React from "react";
import { Download, Eye, Plus, Save, Sparkles, Wand2 } from "lucide-react";
import {
  Excalidraw,
  convertToExcalidrawElements,
  exportToBlob,
} from "@excalidraw/excalidraw";
import type { ExcalidrawImperativeAPI } from "@excalidraw/excalidraw/types";
import { parseMermaidToExcalidraw } from "@excalidraw/mermaid-to-excalidraw";
import "@excalidraw/excalidraw/index.css";

import { HudPanel } from "@/components/HudPanel";
import { MermaidBlock } from "@/components/MermaidBlock";
import { Topbar } from "@/components/Topbar";
import {
  api,
  type Diagram,
  type DiagramPayload,
  type DiagramType,
} from "@/lib/api";

const EMPTY_SCENE = {
  elements: [],
  appState: { theme: "dark" as const },
  files: {},
};

function rel(ts?: number): string {
  if (!ts) return "—";
  const s = Math.max(0, Math.floor(Date.now() / 1000) - ts);
  if (s < 60) return "ahora";
  if (s < 3600) return `${Math.floor(s / 60)}m`;
  if (s < 86400) return `${Math.floor(s / 3600)}h`;
  return `${Math.floor(s / 86400)}d`;
}

export function Diagrams() {
  const excalidrawRef = React.useRef<ExcalidrawImperativeAPI | null>(null);
  const sceneRef = React.useRef<{
    elements: ReturnType<ExcalidrawImperativeAPI["getSceneElements"]>;
    appState: ReturnType<ExcalidrawImperativeAPI["getAppState"]>;
    files: ReturnType<ExcalidrawImperativeAPI["getFiles"]>;
  }>({
    elements: [],
    appState: {} as ReturnType<ExcalidrawImperativeAPI["getAppState"]>,
    files: {},
  });

  const [diagrams, setDiagrams] = React.useState<Diagram[]>([]);
  const [selectedId, setSelectedId] = React.useState<number | null>(null);
  const [title, setTitle] = React.useState("Nuevo diagrama");
  const [prompt, setPrompt] = React.useState(
    "diagrama de arquitectura de AgentHub",
  );
  const [type, setType] = React.useState<DiagramType | "auto">("auto");
  const [mermaid, setMermaid] = React.useState(
    "flowchart LR\n  User[Usuario] --> UI[AgentHub UI]\n  UI --> API[Go API]\n  API --> DB[(SQLite)]",
  );
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [mermaidOpen, setMermaidOpen] = React.useState(false);

  const selected = diagrams.find((d) => d.id === selectedId) ?? null;

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

  async function applyMermaid(src = mermaid) {
    const raw = src.trim();
    if (!raw) return;
    setBusy(true);
    try {
      const parsed = await parseMermaidToExcalidraw(raw, {
        startOnLoad: false,
        themeVariables: { fontSize: "20px" },
      });
      const elements = convertToExcalidrawElements(parsed.elements);
      if (parsed.files) {
        excalidrawRef.current?.addFiles(Object.values(parsed.files));
      }
      excalidrawRef.current?.updateScene({
        elements,
        appState: { theme: "dark", viewBackgroundColor: "#060814" },
      });
      excalidrawRef.current?.scrollToContent(elements, { fitToContent: true });
      setMermaid(raw);
      setError(null);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : "no se pudo convertir Mermaid",
      );
    } finally {
      setBusy(false);
    }
  }

  async function generate(mode: "new" | "improve") {
    const base = prompt.trim();
    if (!base) {
      setError("prompt requerido");
      return;
    }
    setBusy(true);
    try {
      const body =
        mode === "improve" && mermaid.trim()
          ? `${base}\n\nMejorá este diagrama y devolvé una versión más clara:\n${mermaid}`
          : base;
      const res = await api.generateDiagram({
        prompt: body,
        type: type === "auto" ? undefined : type,
      });
      setTitle((t) => (t === "Nuevo diagrama" || !t.trim() ? res.title : t));
      setMermaid(res.mermaid);
      await applyMermaid(res.mermaid);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error generando diagrama");
    } finally {
      setBusy(false);
    }
  }

  function newDiagram() {
    setSelectedId(null);
    setTitle("Nuevo diagrama");
    setPrompt("diagrama de arquitectura de AgentHub");
    setMermaid(
      "flowchart LR\n  User[Usuario] --> UI[AgentHub UI]\n  UI --> API[Go API]\n  API --> DB[(SQLite)]",
    );
    excalidrawRef.current?.resetScene();
  }

  function loadDiagram(d: Diagram) {
    setSelectedId(d.id);
    setTitle(d.title);
    setPrompt(d.prompt ?? "");
    const mer = d.mermaid_source || d.mermaid || "";
    setMermaid(mer);

    // Try the persisted Excalidraw scene first; fall back to rendering from
    // mermaid when (a) the JSON is invalid, or (b) the scene has zero
    // elements but we DO have mermaid source — common case when a diagram
    // was queued from the API without going through the canvas first.
    let parsedScene: {
      elements?: ReturnType<ExcalidrawImperativeAPI["getSceneElements"]>;
      appState?: Partial<ReturnType<ExcalidrawImperativeAPI["getAppState"]>>;
      files?: ReturnType<ExcalidrawImperativeAPI["getFiles"]>;
    } | null = null;
    try {
      parsedScene = JSON.parse(d.excalidraw_json);
    } catch {
      parsedScene = null;
    }
    const hasElements =
      parsedScene && Array.isArray(parsedScene.elements) && parsedScene.elements.length > 0;
    if (hasElements) {
      if (parsedScene!.files)
        excalidrawRef.current?.addFiles(Object.values(parsedScene!.files));
      excalidrawRef.current?.updateScene({
        elements: parsedScene!.elements ?? [],
        appState: {
          ...(parsedScene!.appState ?? {}),
          theme: "dark",
        } as ReturnType<ExcalidrawImperativeAPI["getAppState"]>,
      });
    } else if (mer.trim()) {
      void applyMermaid(mer);
    } else {
      excalidrawRef.current?.resetScene();
    }
  }

  async function save() {
    const cleanTitle = title.trim();
    if (!cleanTitle) {
      setError("title requerido");
      return;
    }
    const current = excalidrawRef.current;
    const scene = current
      ? {
          elements: current.getSceneElements(),
          appState: current.getAppState(),
          files: current.getFiles(),
        }
      : sceneRef.current;
    const payload: DiagramPayload = {
      title: cleanTitle,
      prompt: prompt.trim(),
      mermaid,
      excalidraw_json: JSON.stringify(scene),
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

  async function exportPNG() {
    const current = excalidrawRef.current;
    if (!current) return;
    setBusy(true);
    try {
      const blob = await exportToBlob({
        elements: current.getSceneElements(),
        appState: {
          ...current.getAppState(),
          exportBackground: true,
          viewBackgroundColor: "#060814",
        },
        files: current.getFiles(),
        mimeType: "image/png",
        exportPadding: 24,
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = `${title.trim() || "diagram"}.png`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (err) {
      setError(err instanceof Error ? err.message : "error exportando PNG");
    } finally {
      setBusy(false);
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <Topbar
        breadcrumb={[{ label: "AgentHub" }, { label: "Diagramas" }]}
        status={
          error
            ? { label: "ERROR", tone: "danger" }
            : { label: busy ? "BUSY" : "READY", tone: busy ? "warn" : "ok" }
        }
        right={
          <button
            onClick={() => void save()}
            className="clip-tag px-3 py-1 font-mono text-[10px] uppercase tracking-hud cursor-pointer"
            style={{
              color: "var(--color-lime)",
              border: "1px solid var(--color-lime)",
              background: "rgba(163,255,78,0.10)",
            }}
          >
            guardar
          </button>
        }
      />

      <div className="grid min-h-0 flex-1 grid-cols-[280px_1fr] gap-4 p-4">
        <HudPanel
          title="mis diagramas"
          sub={`${diagrams.length} guardados`}
          accent="cyan"
          className="min-h-0"
        >
          <button
            onClick={newDiagram}
            className="mb-3 flex w-full items-center gap-2 px-3 py-2 clip-tag font-mono text-[10px] uppercase tracking-hud-tight cursor-pointer"
            style={{
              color: "var(--color-lime)",
              border: "1px solid rgba(163,255,78,0.45)",
              background: "rgba(163,255,78,0.08)",
            }}
          >
            <Plus size={13} /> Nuevo
          </button>
          <div className="min-h-0 flex-1 overflow-y-auto pr-1">
            {diagrams.map((d) => {
              const active = d.id === selectedId;
              return (
                <button
                  key={d.id}
                  onClick={() => loadDiagram(d)}
                  className="mb-2 w-full text-left px-3 py-2 clip-hud-sm font-mono cursor-pointer"
                  style={{
                    background: active
                      ? "rgba(94,240,255,0.10)"
                      : "rgba(255,255,255,0.03)",
                    border: `1px solid ${active ? "var(--color-cyan)" : "var(--color-line)"}`,
                  }}
                >
                  <div className="text-[11px] text-[var(--color-fg)] truncate">
                    {d.title}
                  </div>
                  <div className="mt-1 text-[9px] text-[var(--color-dim)]">
                    upd {rel(d.updated_at)}
                  </div>
                </button>
              );
            })}
          </div>
        </HudPanel>

        <div className="grid min-h-0 grid-rows-[1fr_230px] gap-4">
          <div
            className="min-h-0 overflow-hidden clip-hud border bg-black/30"
            style={{ borderColor: "var(--color-line)" }}
          >
            <Excalidraw
              theme="dark"
              initialData={EMPTY_SCENE}
              excalidrawAPI={(apiRef) => {
                excalidrawRef.current = apiRef;
              }}
              onChange={(elements, appState, files) => {
                sceneRef.current = { elements, appState, files };
              }}
              UIOptions={{
                canvasActions: {
                  loadScene: false,
                  saveAsImage: false,
                  export: false,
                },
              }}
            />
          </div>

          <HudPanel
            title={selected ? `prompt panel · #${selected.id}` : "prompt panel"}
            sub="Mermaid → Excalidraw"
            accent="lime"
            className="min-h-0"
          >
            <div className="grid grid-cols-[1fr_190px] gap-3">
              <div className="min-w-0">
                <input
                  value={title}
                  onChange={(e) => setTitle(e.target.value)}
                  className="mb-2 w-full bg-transparent px-3 py-2 clip-tag font-mono text-[12px] outline-none"
                  style={{ border: "1px solid var(--color-line)" }}
                  placeholder="Título"
                />
                <textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  className="h-[112px] w-full resize-none bg-transparent px-3 py-2 clip-hud-sm font-mono text-[12px] outline-none"
                  style={{ border: "1px solid var(--color-line)" }}
                  placeholder="Pedile un diagrama al diagram-architect…"
                />
              </div>
              <div className="flex flex-col gap-2">
                <select
                  value={type}
                  onChange={(e) =>
                    setType(e.target.value as DiagramType | "auto")
                  }
                  className="bg-[var(--color-bg-2)] px-2 py-1 clip-tag font-mono text-[10px]"
                  style={{ border: "1px solid var(--color-line)" }}
                >
                  <option value="auto">auto</option>
                  <option value="flowchart">flowchart</option>
                  <option value="sequence">sequence</option>
                  <option value="c4">c4</option>
                  <option value="erd">erd</option>
                  <option value="mindmap">mindmap</option>
                </select>
                <Action
                  icon={<Sparkles size={13} />}
                  label="Generar"
                  onClick={() => void generate("new")}
                />
                <Action
                  icon={<Wand2 size={13} />}
                  label="Mejorar"
                  onClick={() => void generate("improve")}
                />
                <Action
                  label="Aplicar al canvas"
                  onClick={() => void applyMermaid()}
                />
                <Action
                  icon={<Eye size={13} />}
                  label="Ver Mermaid"
                  onClick={() => setMermaidOpen(true)}
                />
                <Action
                  icon={<Download size={13} />}
                  label="Exportar PNG"
                  onClick={() => void exportPNG()}
                />
                <Action
                  icon={<Save size={13} />}
                  label="Guardar"
                  onClick={() => void save()}
                />
              </div>
            </div>
            {error && (
              <div className="mt-2 font-mono text-[10px] text-[var(--color-danger)]">
                ✗ {error}
              </div>
            )}
          </HudPanel>
        </div>
      </div>

      {mermaidOpen && (
        <MermaidModal
          value={mermaid}
          onChange={setMermaid}
          onClose={() => setMermaidOpen(false)}
          onApply={(value) => {
            setMermaid(value);
            setMermaidOpen(false);
            void applyMermaid(value);
          }}
        />
      )}
    </div>
  );
}

function Action({
  label,
  icon,
  onClick,
}: {
  label: string;
  icon?: React.ReactNode;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-center gap-2 px-2 py-1 clip-tag font-mono text-[10px] uppercase tracking-hud-tight cursor-pointer text-left"
      style={{
        color: "var(--color-cyan)",
        border: "1px solid rgba(94,240,255,0.30)",
        background: "rgba(94,240,255,0.05)",
      }}
    >
      {icon}
      <span>{label}</span>
    </button>
  );
}

function MermaidModal({
  value,
  onChange,
  onClose,
  onApply,
}: {
  value: string;
  onChange: (v: string) => void;
  onClose: () => void;
  onApply: (v: string) => void;
}) {
  const [draft, setDraft] = React.useState(value);
  return (
    <div className="fixed inset-0 z-50 grid grid-cols-[1fr_1fr] gap-4 bg-black/80 p-8">
      <HudPanel
        title="Mermaid editable"
        sub="fuente intermedia"
        accent="cyan"
        className="min-h-0"
      >
        <textarea
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          className="min-h-0 flex-1 resize-none bg-transparent p-3 clip-hud-sm font-mono text-[12px] outline-none"
          style={{ border: "1px solid var(--color-line)" }}
        />
        <div className="mt-3 flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-3 py-1 clip-tag font-mono text-[10px] cursor-pointer"
            style={{
              border: "1px solid var(--color-line)",
              color: "var(--color-dim)",
            }}
          >
            cerrar
          </button>
          <button
            onClick={() => {
              onChange(draft);
              onApply(draft);
            }}
            className="px-3 py-1 clip-tag font-mono text-[10px] cursor-pointer"
            style={{
              border: "1px solid var(--color-lime)",
              color: "var(--color-lime)",
              background: "rgba(163,255,78,0.10)",
            }}
          >
            aplicar al canvas
          </button>
        </div>
      </HudPanel>
      <HudPanel
        title="preview"
        sub="mermaid render"
        accent="magenta"
        className="min-h-0 overflow-y-auto"
      >
        <MermaidBlock content={draft} />
      </HudPanel>
    </div>
  );
}
