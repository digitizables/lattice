import path from "node:path";
import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const docsRoot = path.resolve(__dirname, "../../../docs");

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      "@docs": docsRoot,
    },
  },
  server: {
    port: 5173,
    fs: {
      allow: [path.resolve(__dirname, "../../.."), docsRoot],
    },
    proxy: {
      "/lattice.operator.v1.OperatorService": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
});
