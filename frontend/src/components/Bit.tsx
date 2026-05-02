// BIT — pixel-art notification assistant (Sprint C of 0.5.0).
//
// 4 sprite states (idle / happy / excited / sleep) sourced from /public/bit/.
// Mounted once at the app root (AppShell). Reacts to the NotifCenter context:
//
//   - new unread notif        → state="excited", run to the bell, bubble with
//                                kind/title + a "abrir drawer" button
//   - all read                → state="idle", drift back to the corner
//   - 35s with no activity    → state="sleep", small zZ bubble
//   - drag & drop             → free positioning, persists in localStorage
//
// Performance: a single absolute-positioned img + bubble. No canvas, no rAF
// loop except during transitions (CSS handles it).

import * as React from "react";
import { useNotifCenter } from "@/components/NotifCenter";

type BitState = "idle" | "happy" | "excited" | "sleep";

const SPRITES: Record<BitState, string> = {
  idle:    "/bit/bit-idle.png",
  happy:   "/bit/bit-happy.png",
  excited: "/bit/bit-excited.png",
  sleep:   "/bit/bit-sleep.png",
};

const POS_KEY = "agenthub.bit.pos";
const HIDDEN_KEY = "agenthub.bit.hidden";
const SLEEP_AFTER_MS = 35_000;
const BIT_SIZE = 96;

interface Pos { x: number; y: number; }

function loadPos(): Pos {
  try {
    const raw = localStorage.getItem(POS_KEY);
    if (raw) return JSON.parse(raw) as Pos;
  } catch { /* ignore */ }
  // Default: bottom-right corner.
  return { x: window.innerWidth - BIT_SIZE - 24, y: window.innerHeight - BIT_SIZE - 24 };
}

function savePos(p: Pos) {
  try { localStorage.setItem(POS_KEY, JSON.stringify(p)); } catch { /* ignore */ }
}

export function Bit() {
  const { unread, items, open: openDrawer } = useNotifCenter();
  const [hidden, setHidden] = React.useState<boolean>(() => {
    try { return localStorage.getItem(HIDDEN_KEY) === "1"; } catch { return false; }
  });
  const [pos, setPos] = React.useState<Pos>(() => loadPos());
  const [state, setState] = React.useState<BitState>("idle");
  const [bubble, setBubble] = React.useState<{ title: string; kind: string; cta?: boolean } | null>(null);
  const dragging = React.useRef<{ dx: number; dy: number } | null>(null);
  const lastEventRef = React.useRef<number>(Date.now());

  // ─── react to incoming notifications ─────────────────────────────
  const lastSeenIdRef = React.useRef<string | null>(null);
  React.useEffect(() => {
    const newest = items[0];
    if (!newest) return;
    if (lastSeenIdRef.current === newest.id) return;
    lastSeenIdRef.current = newest.id;
    if (newest.read) return; // initial REST load doesn't trigger excitement

    setState("excited");
    setBubble({ title: newest.title, kind: newest.kind, cta: true });
    lastEventRef.current = Date.now();

    const t1 = window.setTimeout(() => setState("happy"), 1500);
    const t2 = window.setTimeout(() => setBubble(null), 5500);
    return () => { window.clearTimeout(t1); window.clearTimeout(t2); };
  }, [items]);

  // ─── settle to idle when no unread + no recent event ─────────────
  React.useEffect(() => {
    if (unread === 0 && state === "happy") {
      const t = window.setTimeout(() => setState("idle"), 1200);
      return () => window.clearTimeout(t);
    }
  }, [unread, state]);

  // ─── sleep timer ─────────────────────────────────────────────────
  React.useEffect(() => {
    if (state !== "idle") return;
    const handle = window.setInterval(() => {
      if (Date.now() - lastEventRef.current > SLEEP_AFTER_MS) {
        setState("sleep");
        setBubble({ title: "zzZ", kind: "monitor activo", cta: false });
        window.setTimeout(() => setBubble(null), 2500);
      }
    }, 5000);
    return () => window.clearInterval(handle);
  }, [state]);

  // ─── drag handling ───────────────────────────────────────────────
  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    e.currentTarget.setPointerCapture(e.pointerId);
    dragging.current = { dx: e.clientX - pos.x, dy: e.clientY - pos.y };
    lastEventRef.current = Date.now();
    if (state === "sleep") setState("idle");
  };
  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const d = dragging.current;
    if (!d) return;
    const x = Math.max(0, Math.min(window.innerWidth - BIT_SIZE, e.clientX - d.dx));
    const y = Math.max(0, Math.min(window.innerHeight - BIT_SIZE, e.clientY - d.dy));
    setPos({ x, y });
  };
  const onPointerUp = () => {
    if (dragging.current) {
      savePos(pos);
      dragging.current = null;
    }
  };

  const onClick = () => {
    if (dragging.current) return; // suppress during drag end
    lastEventRef.current = Date.now();
    if (unread > 0) {
      openDrawer();
    } else {
      setState("happy");
      setBubble({ title: "qué onda", kind: "estoy acá", cta: false });
      window.setTimeout(() => setBubble(null), 2200);
      window.setTimeout(() => setState("idle"), 2400);
    }
  };

  if (hidden) {
    return (
      <button
        type="button"
        onClick={() => {
          setHidden(false);
          try { localStorage.setItem(HIDDEN_KEY, "0"); } catch { /* ignore */ }
        }}
        title="Mostrar BIT"
        style={{
          position: "fixed",
          bottom: 8, right: 8,
          width: 24, height: 24,
          background: "rgba(94,240,255,0.10)",
          border: "1px solid var(--color-cyan)",
          color: "var(--color-cyan)",
          fontSize: 12,
          zIndex: 70,
          cursor: "pointer",
        }}
      >
        ◆
      </button>
    );
  }

  return (
    <>
      <div
        className={state === "excited" ? "anim-bounce" : undefined}
        style={{
          position: "fixed",
          left: pos.x, top: pos.y,
          width: BIT_SIZE, height: BIT_SIZE,
          zIndex: 70,
          cursor: dragging.current ? "grabbing" : "grab",
          touchAction: "none",
          imageRendering: "pixelated",
          filter: state === "excited"
            ? "drop-shadow(0 0 8px rgba(255,159,67,0.6))"
            : "drop-shadow(0 4px 8px rgba(0,0,0,0.4))",
          transition: dragging.current ? "none" : "left .3s ease-out, top .3s ease-out, filter .3s",
        }}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={onPointerUp}
        onPointerCancel={onPointerUp}
        onClick={onClick}
        title={state === "sleep" ? "BIT durmiendo · click para despertar" : "BIT"}
      >
        <img
          src={SPRITES[state]}
          alt={`bit-${state}`}
          width={BIT_SIZE}
          height={BIT_SIZE}
          draggable={false}
          style={{ display: "block", width: "100%", height: "100%", userSelect: "none" }}
        />
      </div>

      {bubble && (
        <BitBubble
          x={pos.x + BIT_SIZE - 8}
          y={pos.y - 6}
          title={bubble.title}
          kind={bubble.kind}
          accent={state === "excited" ? "var(--color-orange)" : "var(--color-cyan)"}
          onAction={bubble.cta ? () => { openDrawer(); setBubble(null); } : undefined}
        />
      )}

      {/* Hidden hide-me button on long-press not implemented; quick toggle via context-menu corner button */}
      <button
        type="button"
        onClick={() => {
          setHidden(true);
          try { localStorage.setItem(HIDDEN_KEY, "1"); } catch { /* ignore */ }
        }}
        title="Ocultar BIT"
        style={{
          position: "fixed",
          left: pos.x + BIT_SIZE - 14, top: pos.y - 6,
          width: 18, height: 18,
          padding: 0,
          background: "rgba(10,15,36,0.85)",
          color: "var(--color-dim)",
          border: "1px solid var(--color-line)",
          fontSize: 10,
          cursor: "pointer",
          zIndex: 71,
          opacity: dragging.current ? 0 : 0.55,
          transition: "opacity .2s",
        }}
      >
        ×
      </button>
    </>
  );
}

function BitBubble({
  x, y, title, kind, accent, onAction,
}: {
  x: number; y: number; title: string; kind: string; accent: string; onAction?: () => void;
}) {
  return (
    <div
      className="anim-fade-in-up"
      style={{
        position: "fixed",
        left: x + 4, top: y,
        zIndex: 71,
        maxWidth: 260,
        background: "rgba(10,15,36,0.96)",
        border: `1px solid ${accent}`,
        clipPath: "polygon(10px 0, 100% 0, 100% calc(100% - 10px), calc(100% - 10px) 100%, 0 100%, 0 10px)",
        padding: "8px 12px",
        pointerEvents: "auto",
      }}
    >
      <div className="font-mono" style={{ fontSize: 9, letterSpacing: "0.12em", textTransform: "uppercase", color: "var(--color-magenta)" }}>
        {kind}
      </div>
      <div style={{ color: "var(--color-fg)", fontSize: 11, fontWeight: 600, marginTop: 2, lineHeight: 1.3 }}>
        {title}
      </div>
      {onAction && (
        <button
          type="button"
          onClick={onAction}
          className="cursor-pointer mt-2 clip-tag"
          style={{
            padding: "3px 10px",
            fontSize: 9,
            letterSpacing: "0.18em",
            textTransform: "uppercase",
            color: accent,
            border: `1px solid ${accent}`,
            background: `${accent}1a`,
          }}
        >
          abrir drawer ▸
        </button>
      )}
    </div>
  );
}
