import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'

export default defineConfig({
  plugins: [vue()],
  // Read VITE_* variables from the repo-root .env shared with the backend.
  envDir: '..',
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
    // Telegram needs an HTTPS URL; expose the dev server through a tunnel.
    allowedHosts: ['.trycloudflare.com', '.ngrok-free.app'],
    proxy: {
      // The API, and the server-rendered public website it also serves.
      '^/(api|t|c)/': { target: 'http://localhost:8080', changeOrigin: true },
      '/sitemap.xml': { target: 'http://localhost:8080', changeOrigin: true },
      '/robots.txt': { target: 'http://localhost:8080', changeOrigin: true },
    },
  },
})
