import { defineConfig } from 'vitest/config';
import preact from '@preact/preset-vite';

// `just dev` runs the Go server on API_URL and this dev server in front of it;
// `npm run build` writes the files the Go binary embeds (internal/web/dist).
const api = process.env.API_URL ?? 'http://127.0.0.1:8080';

export default defineConfig({
  plugins: [preact()],
  build: {
    outDir: '../internal/web/dist',
    emptyOutDir: false, // scripts/clean-dist.mjs does it, keeping the .gitkeep placeholder
    target: 'es2022',
  },
  server: {
    proxy: { '/api': { target: api, changeOrigin: false } },
  },
  test: { environment: 'node', include: ['src/**/*.test.ts'] },
});
