import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'node:path'

// Development-only fixture: real gallery/dialog/renderers; synthetic API and app shell.
export default defineConfig({
  plugins: [vue()],
  resolve: { alias: [
    { find: '@/components/layout/AppLayout.vue', replacement: resolve(__dirname, 'browser/pelican-showcase/Layout.vue') },
    { find: '@/api/pelicanShowcase', replacement: resolve(__dirname, 'browser/pelican-showcase/api.ts') },
    { find: '@/stores/auth', replacement: resolve(__dirname, 'browser/pelican-showcase/stores.ts') },
    { find: '@/stores/app', replacement: resolve(__dirname, 'browser/pelican-showcase/stores.ts') },
    { find: '@', replacement: resolve(__dirname, 'src') },
  ] },
  define: { __INTLIFY_JIT_COMPILATION__: true },
  build: { rollupOptions: { input: resolve(__dirname, 'browser/pelican-showcase/index.html') } },
  server: { host: '127.0.0.1', port: 8771, strictPort: true, cors: false },
})
