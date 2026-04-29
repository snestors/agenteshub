import { initializeApp } from "firebase/app";
import { getMessaging, getToken, isSupported, onMessage, type Messaging } from "firebase/messaging";
import { api } from "@/lib/api";

const firebaseConfig = {
  apiKey: "AIzaSyDtnUHyEAkB6h9qCeTAtIrtWNpG7flEkAk",
  authDomain: "relogtemperatura.firebaseapp.com",
  databaseURL: "https://relogtemperatura-default-rtdb.firebaseio.com",
  projectId: "relogtemperatura",
  storageBucket: "relogtemperatura.firebasestorage.app",
  messagingSenderId: "100530365913",
  appId: "1:100530365913:web:096380327c9c7151e265c8",
};

// RelogTemperatura todavía no expone VAPID custom por CLI. Firebase puede usar
// la key default del proyecto; si se agrega una Web Push certificate en Console,
// se puede completar esta constante sin tocar el flujo.
const vapidKey = "";

let messagingPromise: Promise<Messaging | null> | null = null;
let registeredToken: string | null = null;
let foregroundListenerStarted = false;

export type FirebasePushResult =
  | "registered"
  | "unsupported"
  | "denied"
  | "dismissed"
  | "no-token";

export type FirebasePushSupport = {
  ok: boolean;
  reason?: string;
  message?: string;
};

async function messaging(): Promise<Messaging | null> {
  if (!messagingPromise) {
    messagingPromise = (async () => {
      if (!(await isSupported())) return null;
      const app = initializeApp(firebaseConfig);
      return getMessaging(app);
    })();
  }
  return messagingPromise;
}

async function serviceWorkerReady(): Promise<ServiceWorkerRegistration | null> {
  return Promise.race<ServiceWorkerRegistration | null>([
    navigator.serviceWorker.ready,
    new Promise((resolve) => window.setTimeout(() => resolve(null), 10000)),
  ]);
}

export async function getFirebasePushSupport(): Promise<FirebasePushSupport> {
  if (!("Notification" in window)) {
    return { ok: false, reason: "notification-api", message: "Este navegador no expone permisos de notificación." };
  }
  if (!window.isSecureContext) {
    return { ok: false, reason: "insecure", message: "Abrilo por HTTPS para activar push." };
  }
  if (!("serviceWorker" in navigator)) {
    return { ok: false, reason: "service-worker", message: "Este navegador no soporta service workers." };
  }
  if (!("PushManager" in window)) {
    return { ok: false, reason: "push-manager", message: "Este navegador no soporta Web Push." };
  }
  const m = await messaging();
  if (!m) {
    return { ok: false, reason: "firebase", message: "Firebase Messaging no está soportado en este contexto." };
  }
  return { ok: true };
}

export async function registerFirebasePush(): Promise<FirebasePushResult> {
  const support = await getFirebasePushSupport();
  if (!support.ok) return "unsupported";
  const m = await messaging();
  if (!m) return "unsupported";

  let permission = Notification.permission;
  if (permission === "default") {
    try {
      permission = await Notification.requestPermission();
    } catch {
      return "dismissed";
    }
  }
  if (permission === "default") return "dismissed";
  if (permission !== "granted") return "denied";

  const sw = await serviceWorkerReady();
  if (!sw) return "unsupported";
  const token = await getToken(m, {
    serviceWorkerRegistration: sw,
    ...(vapidKey ? { vapidKey } : {}),
  });
  if (!token) return "no-token";
  if (token !== registeredToken) {
    await api.registerPushToken(token);
    registeredToken = token;
  }

  if (!foregroundListenerStarted) {
    foregroundListenerStarted = true;
    onMessage(m, (payload) => {
      if (payload.notification?.title) {
        // El WS ya muestra el toast in-app. Esto queda como fallback suave si FCM
        // llega en foreground sin duplicar fuerte.
        console.debug("[fcm] foreground", payload.notification.title);
      }
    });
  }

  return "registered";
}

export async function registerFirebasePushIfGranted(): Promise<boolean> {
  if (!("Notification" in window) || Notification.permission !== "granted") return false;
  try {
    return (await registerFirebasePush()) === "registered";
  } catch {
    // best effort: la UI tiene botón manual en el drawer
    return false;
  }
}
