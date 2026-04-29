export const PROJECT_SESSION_CONFIG_EVENT = "agenthub:toggle-project-session-config";

export function toggleProjectSessionConfig(sessionId: number) {
  window.dispatchEvent(
    new CustomEvent(PROJECT_SESSION_CONFIG_EVENT, { detail: { sessionId } }),
  );
}
