export const PROJECT_SESSION_CONFIG_EVENT = "agenthub:toggle-project-session-config";
export const PROJECT_SESSION_CANCEL_EVENT = "agenthub:cancel-project-session";

export function toggleProjectSessionConfig(sessionId: number) {
  window.dispatchEvent(
    new CustomEvent(PROJECT_SESSION_CONFIG_EVENT, { detail: { sessionId } }),
  );
}

export function cancelProjectSession(sessionId: number) {
  window.dispatchEvent(
    new CustomEvent(PROJECT_SESSION_CANCEL_EVENT, { detail: { sessionId } }),
  );
}
