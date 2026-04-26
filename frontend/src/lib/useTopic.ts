// React hook that subscribes a component to a topic on the singleton wsClient.
//
// Re-renders are minimal: the onMessage callback is held in a ref so that
// changing it across renders does NOT re-subscribe (which would send an
// unsubscribe + subscribe pair). The effect only re-runs when `topic` or
// `enabled` change.
//
// `status` is exposed so consumers can mirror their fallback/polling logic
// to the singleton's transport state.

import * as React from "react";
import { wsClient, type WsEnvelope, type WsStatus } from "./wsClient";

export type { WsEnvelope, WsStatus };

export type TopicListener<T> = (payload: T, envelope: WsEnvelope) => void;

export function useTopic<T = unknown>(
  topic: string,
  onMessage: TopicListener<T>,
  enabled: boolean = true,
): { status: WsStatus } {
  const onMessageRef = React.useRef(onMessage);
  React.useEffect(() => {
    onMessageRef.current = onMessage;
  });

  const [status, setStatus] = React.useState<WsStatus>(() => wsClient.status());

  React.useEffect(() => {
    // mirror singleton status into local state
    setStatus(wsClient.status());
    const unsub = wsClient.onStatusChange(setStatus);
    return unsub;
  }, []);

  React.useEffect(() => {
    if (!enabled) return;
    const unsubscribe = wsClient.subscribe(topic, (envelope) => {
      onMessageRef.current(envelope.payload as T, envelope);
    });
    return unsubscribe;
  }, [topic, enabled]);

  return { status };
}
