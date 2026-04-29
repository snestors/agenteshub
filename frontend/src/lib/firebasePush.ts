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

export async function registerFirebasePush(): Promise<"registered" | "unsupported" | "denied" | "no-token"> {
  if (!("Notification" in window) || !("serviceWorker" in navigator)) return "unsupported";
  const m = await messaging();
  if (!m) return "unsupported";

  let permission = Notification.permission;
  if (permission === "default") {
    permission = await Notification.requestPermission();
  }
  if (permission !== "granted") return "denied";

  const sw = await navigator.serviceWorker.ready;
  const token = await getToken(m, {
    serviceWorkerRegistration: sw,
    ...(vapidKey ? { vapidKey } : {}),
  });
  if (!token) return "no-token";
  if (token !== registeredToken) {
    await api.registerPushToken(token);
    registeredToken = token;
  }

  onMessage(m, (payload) => {
    if (payload.notification?.title) {
      // El WS ya muestra el toast in-app. Esto queda como fallback suave si FCM
      // llega en foreground sin duplicar fuerte.
      console.debug("[fcm] foreground", payload.notification.title);
    }
  });

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
