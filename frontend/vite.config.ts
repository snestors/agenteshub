import path from "node:path";
import fs from "node:fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

const appVersion = fs.readFileSync(path.resolve(__dirname, "../VERSION"), "utf-8").trim();

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  define: {
    __APP_VERSION__: JSON.stringify(appVersion),
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://localhost:8093",
        changeOrigin: true,
      },
      "/healthz": {
        target: "http://localhost:8093",
        changeOrigin: true,
      },
      "/ws": {
        target: "ws://localhost:8093",
        ws: true,
        changeOrigin: true,
      },
    },
  },
});
