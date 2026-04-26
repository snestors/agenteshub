// Cross-navigation streams store.
//
// Ghost bubbles (live tool/text/thinking chunks) used to live as local state
// in ChatMain. Result: navigating away during a turn lost the ghost; coming
// back showed only the persisted final message. This store keeps the
// in-flight ghosts alive at app scope, so any consumer that mounts later
// sees the ongoing stream.
//
// V1 scope: subscribes globally to the "agent" topic (main-agent chat).
// Project sessions still own their own state for now — promote them here
// when the same UX gap appears.

import * as React from "react";
import { useTopic, type WsEnvelope } from "@/lib/useTopic";
import type { GhostBubbleData, ToolCall } from "@/components/GhostBubble";

type StreamKind = "text" | "tool_use" | "tool_result" | "thinking";

export interface WsStreamPayload {
  kind: StreamKind;
  text?: string;
  tool_name?: string;
  tool_args?: unknown;
  tool_result?: string;
  session_id: string;
  seq: number;
  final?: boolean;
}

type GhostsByTopic = Record<string, Record<string, GhostBubbleData>>;

interface StreamsState {
  /** Ghosts of the agent topic, indexed by session_id. */
  agentGhosts: Record<string, GhostBubbleData>;
  /** Ordered list (Object.values) — convenient for rendering. */
  agentGhostsList: GhostBubbleData[];
}

const Ctx = React.createContext<StreamsState | null>(null);

function parsePayload(payload: unknown): WsStreamPayload | null {
  if (!payload) return null;
  if (typeof payload === "string") {
    try {
      const obj = JSON.parse(payload);
      return obj && typeof obj === "object" ? (obj as WsStreamPayload) : null;
    } catch {
      return null;
    }
  }
  if (typeof payload === "object") return payload as WsStreamPayload;
  return null;
}

function applyChunk(curr: GhostsByTopic, topic: string, chunk: WsStreamPayload): GhostsByTopic {
  const inner = curr[topic] ?? {};
  const sid = chunk.session_id || "default";

  if (chunk.final) {
    if (!(sid in inner)) return curr;
    const nextInner = { ...inner };
    delete nextInner[sid];
    return { ...curr, [topic]: nextInner };
  }

  const existing: GhostBubbleData = inner[sid] ?? {
    id: `stream-${sid}`,
    thinking: "",
    text: "",
    tools: [],
  };

  switch (chunk.kind) {
    case "text": {
      if (!chunk.text) return curr;
      return {
        ...curr,
        [topic]: { ...inner, [sid]: { ...existing, text: existing.text + chunk.text } },
      };
    }
    case "thinking": {
      if (!chunk.text) return curr;
      return {
        ...curr,
        [topic]: { ...inner, [sid]: { ...existing, thinking: existing.thinking + chunk.text } },
      };
    }
    case "tool_use": {
      const id = `${sid}-${chunk.seq}`;
      const newCall: ToolCall = {
        id,
        name: chunk.tool_name ?? "tool",
        args: chunk.tool_args,
        status: "running",
      };
      const tools = existing.tools.some((t) => t.id === id)
        ? existing.tools
        : [...existing.tools, newCall];
      return {
        ...curr,
        [topic]: { ...inner, [sid]: { ...existing, tools } },
      };
    }
    case "tool_result": {
      const tools = existing.tools.slice();
      let idx = -1;
      for (let i = tools.length - 1; i >= 0; i--) {
        if (
          tools[i].status === "running" &&
          (!chunk.tool_name || tools[i].name === chunk.tool_name)
        ) {
          idx = i;
          break;
        }
      }
      if (idx === -1 && tools.length > 0) idx = tools.length - 1;
      if (idx >= 0) {
        const preview = (chunk.tool_result ?? "").slice(0, 200);
        tools[idx] = { ...tools[idx], status: "ok", resultPreview: preview };
      }
      return {
        ...curr,
        [topic]: { ...inner, [sid]: { ...existing, tools } },
      };
    }
    default:
      return curr;
  }
}

export function StreamsProvider({ children }: { children: React.ReactNode }) {
  const [byTopic, setByTopic] = React.useState<GhostsByTopic>({});

  const handleAgent = React.useCallback((_payload: unknown, evt: WsEnvelope) => {
    if (evt.type !== "stream") return;
    const chunk = parsePayload(evt.payload);
    if (!chunk) return;
    setByTopic((curr) => applyChunk(curr, "agent", chunk));
  }, []);

  // Always subscribed at app scope: this is what makes the ghost survive
  // navigating away from /chat during an in-flight turn.
  useTopic("agent", handleAgent);

  const agentGhosts = byTopic.agent ?? {};
  const agentGhostsList = React.useMemo(() => Object.values(agentGhosts), [agentGhosts]);

  const value = React.useMemo<StreamsState>(
    () => ({ agentGhosts, agentGhostsList }),
    [agentGhosts, agentGhostsList]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useStreams(): StreamsState {
  const v = React.useContext(Ctx);
  if (!v) throw new Error("useStreams must be used inside StreamsProvider");
  return v;
}
