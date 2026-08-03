import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

const devProxyTarget = process.env.VITE_DEV_PROXY_TARGET ?? "http://127.0.0.1:8080";

export default defineConfig({
  plugins: [react()],
  optimizeDeps: {
    // Prevent late dependency re-optimization when navigating to install-wizard routes.
    include: ["@tanstack/react-form"],
  },
  resolve: {
    dedupe: ["react", "react-dom"],
    alias: {
      "@": path.resolve(__dirname, "client"),
    },
  },
  server: {
    host: "localhost",
    port: 51273,
    strictPort: true,
    proxy: {
      "/bff": {
        target: devProxyTarget,
        changeOrigin: true,
        rewrite: (path) => {
          const rewrittenPath = path.replace(/^\/bff(?=\/|$)/, "");
          return rewrittenPath || "/";
        },
      },
    },
  },
  // Keep app cache isolated in case another local Vite instance is running (e.g. Ladle).
  cacheDir: "node_modules/.vite-customer-dashboard",
});
