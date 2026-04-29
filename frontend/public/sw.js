const VERSION = "0.2.44";
const CACHE_NAME = `agenthub-pwa-${VERSION}`;

function parsePushPayload(event) {
  if (!event.data) return {};
  try {
    return event.data.json();
  } catch {
    return { notification: { title: "AgentHub", body: event.data.text() } };
  }
}

function notificationFromPayload(payload) {
  const fcm = payload.FCM_MSG || payload.data?.FCM_MSG || payload;
  const parsed = typeof fcm === "string" ? (() => { try { return JSON.parse(fcm); } catch { return {}; } })() : fcm;
  const notification = parsed.notification || payload.notification || {};
  const data = { ...(parsed.data || {}), ...(payload.data || {}) };
  const title = notification.title || data.title || "AgentHub";
  const body = notification.body || data.body || "Nueva notificación";
  const link = parsed.fcmOptions?.link || parsed.webpush?.fcm_options?.link || data.link || "/";
  return {
    title,
    options: {
      body,
      icon: notification.icon || "/pwa-192.png",
      badge: "/pwa-192.png",
      data: { link },
      tag: data.id || parsed.messageId || undefined,
    },
  };
}

self.addEventListener("install", () => {
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  event.waitUntil((async () => {
    const keys = await caches.keys();
    await Promise.all(keys.filter((key) => key.startsWith("agenthub-pwa-") && key !== CACHE_NAME).map((key) => caches.delete(key)));
    await self.clients.claim();
  })());
});

self.addEventListener("fetch", (event) => {
  const request = event.request;
  if (request.method !== "GET") return;
  const url = new URL(request.url);
  if (url.origin !== self.location.origin) return;
  if (url.pathname.startsWith("/api") || url.pathname.startsWith("/ws")) return;

  if (request.mode === "navigate") {
    event.respondWith(fetch(request).catch(() => caches.match("/index.html")));
    return;
  }

  event.respondWith(fetch(request));
});

self.addEventListener("push", (event) => {
  const payload = parsePushPayload(event);
  const { title, options } = notificationFromPayload(payload);
  event.waitUntil(self.registration.showNotification(title, options));
});

self.addEventListener("notificationclick", (event) => {
  event.notification.close();
  const link = event.notification.data?.link || "/";
  const target = new URL(link, self.location.origin).href;
  event.waitUntil((async () => {
    const allClients = await self.clients.matchAll({ type: "window", includeUncontrolled: true });
    for (const client of allClients) {
      if (client.url.startsWith(self.location.origin) && "focus" in client) {
        await client.focus();
        if ("navigate" in client) await client.navigate(target);
        return;
      }
    }
    await self.clients.openWindow(target);
  })());
});
