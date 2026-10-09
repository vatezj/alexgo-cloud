import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5174,
    proxy: {
      // member 前缀可独立上游：mono（默认）两服务同进程 → 全部指 8080；
      // micro 本地联调：MEMBER_PROXY=http://localhost:8081 npm run dev
      // （Vite 代理键按对象顺序匹配，member 两条必须在 /api 之前，spec §2.3）
      '/api/app/member': { target: process.env.MEMBER_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/api/admin/member': { target: process.env.MEMBER_PROXY || 'http://localhost:8080', changeOrigin: true },
      '/api': 'http://localhost:8080',
      '/swagger': 'http://localhost:8080',
    },
  },
})

