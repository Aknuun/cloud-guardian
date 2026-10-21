import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: '/app/',
  build: {
    outDir: '../server/internal/router/webui',
    emptyOutDir: true,
    target: 'es2020',
    assetsInlineLimit: 0,
  },
})
