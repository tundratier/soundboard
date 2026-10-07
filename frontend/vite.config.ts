import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Dev: `npm run dev` → Vite on :5173; API/WS/icons are forwarded to the Go backend (port 8723).
// Note: admin endpoints only accept localhost hosts; Vite also runs on localhost → works.
const backend = 'http://localhost:8723'

export default defineConfig({
  plugins: [react()],
  build: { outDir: '../backend/web/dist', emptyOutDir: true },
  server: {
    proxy: {
      '/api': backend,
      '/icons': backend,
      '/ws': { target: backend, ws: true },
    },
  },
})
