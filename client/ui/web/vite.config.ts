import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      "/lattice.operator.v1.OperatorService": {
        target: "http://127.0.0.1:8080",
        changeOrigin: true,
      },
    },
  },
});
