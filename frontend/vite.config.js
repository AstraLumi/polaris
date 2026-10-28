import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { serviceWorkerSource } from './src/offline/swBuild.js'

// /sw.js, the service worker for offline writing (docs/EDITOR.md, "Offline
// writing"), written from src/offline/sw.js with this build's file list.
// Builds only: the dev server has none.
function serviceWorker() {
  const template = fileURLToPath(new URL('./src/offline/sw.js', import.meta.url))
  return {
    name: 'polaris-service-worker',
    apply: 'build',
    enforce: 'post',
    generateBundle(_, bundle) {
      const { source } = serviceWorkerSource(readFileSync(template, 'utf8'), bundle)
      this.emitFile({ type: 'asset', fileName: 'sw.js', source })
    },
  }
}

export default defineConfig({
  plugins: [vue(), serviceWorker()],
  build: {
    outDir: 'dist',
  },
  // `npm run dev` talks to a backend running on :8091.
  server: {
    proxy: {
      '/api': 'http://localhost:8091',
      '/uploads': 'http://localhost:8091',
    },
  },
})
