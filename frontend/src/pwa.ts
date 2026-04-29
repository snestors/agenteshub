export function registerPwaServiceWorker() {
  if (!("serviceWorker" in navigator)) return;

  window.addEventListener("load", () => {
    navigator.serviceWorker.register("/sw.js?v=0.2.48").catch(() => {
      // No bloquear la UI si el browser/contexto no permite SW (p. ej. HTTP no seguro).
    });
  });
}
