import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath, URL } from "node:url";

// FSD: импорты внутри admin идут через alias "@" -> src.
// Типы API берём из сгенерированного из openapi.yaml файла (alias "@api-types").
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
      "@api-types": fileURLToPath(
        new URL("../../shared/api-types/ts/schema.ts", import.meta.url),
      ),
    },
  },
  server: {
    port: 5173,
    // В dev проксируем API на backend, чтобы не упираться в CORS
    // (в проде admin и api за одним gateway). Базовый URL запросов — "/api/v1".
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
