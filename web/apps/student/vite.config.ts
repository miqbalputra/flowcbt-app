import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5174,
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    // Bundel awal siswa ringan (FR-11.5): satu vendor React saja untuk sekarang.
    rollupOptions: {
      output: {
        manualChunks: { vendor: ["react", "react-dom"] },
      },
    },
  },
});
