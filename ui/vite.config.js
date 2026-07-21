import { defineConfig } from 'vite';
import vue from '@vitejs/plugin-vue';

// Root is this ui/ directory itself; build output goes to ui/dist, which
// ui/embed.go embeds into the Go binary. The dev-server proxy lets
// `npm run dev` run against a locally-running `go run ./cmd/server`
// (or the existing Docker container) without CORS/relative-path issues.
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
});
