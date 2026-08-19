import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

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
  // No dev server and no proxy: `bun run dev` runs `vite build --watch` into
  // dist/, which the Go server serves on :8080 exactly as it does in production.
  // A dev-only proxy would have to duplicate the server's API-vs-client-route
  // classification (see internal/spa.isAPIPath) and would drift from it.
  // Keep app cache isolated in case another local Vite instance is running (e.g. Ladle).
  cacheDir: "node_modules/.vite-customer-dashboard",
});
