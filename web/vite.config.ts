import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import { defineConfig, loadEnv } from 'vite'
import path from 'node:path'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiBase = env.VITE_API_BASE || 'http://localhost:8080'

  return {
    // Shortr serves the built SPA under /app/*, so every asset URL Vite
    // emits must carry that prefix (otherwise /assets/* 404s against the
    // Go server's routing — see spa.go).
    base: '/app/',
    plugins: [react(), tailwindcss()],
    resolve: {
      alias: {
        '@': path.resolve(import.meta.dirname, './src'),
      },
    },
    server: {
      proxy: {
        '/api': { target: apiBase, changeOrigin: true },
        '/auth': { target: apiBase, changeOrigin: true },
        '/healthz': { target: apiBase, changeOrigin: true },
      },
    },
    build: {
      outDir: 'dist',
      sourcemap: false,
    },
  }
})
