import { defineConfig } from "vite";
import { octane } from "@octanejs/vite-plugin";

export default defineConfig({
  plugins: [octane()],
  server: { host: "127.0.0.1", proxy: { "/api": "http://127.0.0.1:17321" } },
  build: { outDir: "../internal/web/static", emptyOutDir: true }
});
