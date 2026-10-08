import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
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
