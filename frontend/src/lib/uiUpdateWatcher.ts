type UiUpdate = {
  current: string;
  next: string;
};

const CHECK_INTERVAL_MS = 60_000;

function normalizeAsset(value: string | null): string | null {
  if (!value) return null;
  try {
    const url = new URL(value, window.location.href);
    if (!url.pathname.startsWith("/assets/")) return null;
    return url.pathname + url.search;
  } catch {
    return null;
  }
}

function signatureFromElements(root: Document): string {
  const scripts = Array.from(root.querySelectorAll<HTMLScriptElement>('script[type="module"][src]'))
    .map((el) => normalizeAsset(el.getAttribute("src")));
  const styles = Array.from(root.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"][href]'))
    .map((el) => normalizeAsset(el.getAttribute("href")));

  return [...scripts, ...styles]
    .filter((v): v is string => Boolean(v))
    .sort()
    .join("|");
}

async function fetchLatestSignature(): Promise<string | null> {
  const res = await fetch(`/?__agenthub_ui_update=${Date.now()}`, {
    cache: "no-store",
    headers: { Accept: "text/html" },
  });
  if (!res.ok) return null;
  const html = await res.text();
  const doc = new DOMParser().parseFromString(html, "text/html");
  return signatureFromElements(doc);
}

export function startUiUpdateWatcher(onUpdate: (update: UiUpdate) => void): () => void {
  const current = signatureFromElements(document);
  if (!current) return () => {};

  let stopped = false;
  let notified = false;

  const check = async () => {
    if (stopped || notified) return;
    try {
      const next = await fetchLatestSignature();
      if (!next || next === current) return;
      notified = true;
      onUpdate({ current, next });
    } catch {
      // Best effort: si la red/túnel falla, reintentamos en el próximo tick.
    }
  };

  const onVisible = () => {
    if (document.visibilityState === "visible") void check();
  };
  const onFocus = () => void check();

  window.setTimeout(() => void check(), 10_000);
  const interval = window.setInterval(() => void check(), CHECK_INTERVAL_MS);
  document.addEventListener("visibilitychange", onVisible);
  window.addEventListener("focus", onFocus);
  window.addEventListener("online", onFocus);

  return () => {
    stopped = true;
    window.clearInterval(interval);
    document.removeEventListener("visibilitychange", onVisible);
    window.removeEventListener("focus", onFocus);
    window.removeEventListener("online", onFocus);
  };
}
