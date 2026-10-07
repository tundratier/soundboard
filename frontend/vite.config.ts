import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// Dev: `npm run dev` → Vite auf :5173, API/WS/Icons werden an das Go-Backend (Port 8723) weitergeleitet.
// Hinweis: Admin-Endpunkte akzeptieren nur localhost-Hosts, Vite läuft ebenfalls auf localhost → passt.
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
