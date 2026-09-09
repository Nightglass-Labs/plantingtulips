import { fileURLToPath } from "node:url";
import { defineConfig } from "vite";
import solid from "@solidjs/vite-plugin";
import stylex from "@stylexjs/unplugin";

const rootDir = fileURLToPath(new URL(".", import.meta.url));

export default defineConfig({
  plugins: [
    stylex.vite({
      devMode: "full",
      useCSSLayers: true,
      unstable_moduleResolution: {
        type: "commonJS",
        rootDir,
      },
    }),
    solid(),
  ],
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/robots.txt": "http://127.0.0.1:8080",
      "/sitemap.xml": "http://127.0.0.1:8080",
    },
  },
  build: {
    target: "es2022",
  },
});
