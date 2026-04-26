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
  /** Add a "pending" ghost for the agent topic the moment the user sends. */
  markAgentPending: () => void;
  /** Drop a ghost once the persisted message has reached the consumer. */
  dismissAgentGhost: (sessionId: string) => void;
}

const PENDING_KEY = "__pending__";

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

  // First real chunk: drop the placeholder if any (it's been promoted to a
  // real ghost keyed by session_id).
  let workingInner = inner;
  if (PENDING_KEY in workingInner) {
    workingInner = { ...workingInner };
    delete workingInner[PENDING_KEY];
  }

  if (chunk.final) {
    // Don't drop yet — keep the ghost visible until the persisted message
    // arrives. ChatMain will call dismissAgentGhost(sid) then.
    const existing: GhostBubbleData = workingInner[sid] ?? {
      id: `stream-${sid}`,
      thinking: "",
      text: "",
      tools: [],
    };
    return {
      ...curr,
      [topic]: { ...workingInner, [sid]: { ...existing, done: true, pending: false } },
    };
  }

  const existing: GhostBubbleData = workingInner[sid] ?? {
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
        [topic]: { ...workingInner, [sid]: { ...existing, pending: false, text: existing.text + chunk.text } },
      };
    }
    case "thinking": {
      if (!chunk.text) return curr;
      return {
        ...curr,
        [topic]: { ...workingInner, [sid]: { ...existing, pending: false, thinking: existing.thinking + chunk.text } },
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
        [topic]: { ...workingInner, [sid]: { ...existing, pending: false, tools } },
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
        [topic]: { ...workingInner, [sid]: { ...existing, pending: false, tools } },
      };
    }
    default:
      return { ...curr, [topic]: workingInner };
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

  const markAgentPending = React.useCallback(() => {
    setByTopic((curr) => {
      const inner = curr.agent ?? {};
      // If there's already a real ghost, don't overwrite it with a placeholder
      const hasReal = Object.keys(inner).some((k) => k !== PENDING_KEY);
      if (hasReal) return curr;
      return {
        ...curr,
        agent: {
          ...inner,
          [PENDING_KEY]: {
            id: "stream-pending",
            thinking: "",
            text: "",
            tools: [],
            pending: true,
          },
        },
      };
    });
  }, []);

  const dismissAgentGhost = React.useCallback((sessionId: string) => {
    setByTopic((curr) => {
      const inner = curr.agent ?? {};
      let next: Record<string, GhostBubbleData> | null = null;
      if (sessionId in inner) {
        next = { ...inner };
        delete next[sessionId];
      }
      // also clear any leftover placeholder
      if (PENDING_KEY in (next ?? inner)) {
        next = next ?? { ...inner };
        delete next[PENDING_KEY];
      }
      if (!next) return curr;
      return { ...curr, agent: next };
    });
  }, []);

  const agentGhosts = byTopic.agent ?? {};
  const agentGhostsList = React.useMemo(() => Object.values(agentGhosts), [agentGhosts]);

  const value = React.useMemo<StreamsState>(
    () => ({ agentGhosts, agentGhostsList, markAgentPending, dismissAgentGhost }),
    [agentGhosts, agentGhostsList, markAgentPending, dismissAgentGhost]
  );

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useStreams(): StreamsState {
  const v = React.useContext(Ctx);
  if (!v) throw new Error("useStreams must be used inside StreamsProvider");
  return v;
}
