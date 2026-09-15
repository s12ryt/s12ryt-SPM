import { defineConfig } from "vitest/config";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  test: { environment: "jsdom" },
  server: { proxy: { "/api": "http://127.0.0.1:8080" } },
});
