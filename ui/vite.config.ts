import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    proxy: {
      '/ready': 'http://127.0.0.1:8765',
      '/capabilities': 'http://127.0.0.1:8765',
      '/turn': 'http://127.0.0.1:8765',
      '/sessions': 'http://127.0.0.1:8765',
      '/materials': 'http://127.0.0.1:8765',
    },
  },
})
