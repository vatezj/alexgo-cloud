import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => {
  return {
    application: {},
    vite: {
      server: {
        proxy: {
          // 会员端独立代理（MEMBER_PROXY 可覆盖）——长前缀必须排在 /api 之前
          '/api/app/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || 'http://localhost:8080',
          },
          '/api/admin/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || 'http://localhost:8080',
          },
          // 业务 API → Go 后端。不 rewrite：vben baseURL=/api，请求已是 /api/**，gin 挂的正是 /api/**
          '/api': {
            changeOrigin: true,
            target: 'http://localhost:8080',
          },
          '/swagger': {
            changeOrigin: true,
            target: 'http://localhost:8080',
          },
        },
      },
    },
  };
});
