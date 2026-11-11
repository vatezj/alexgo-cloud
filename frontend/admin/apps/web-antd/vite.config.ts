import { defineConfig } from '@vben/vite-config';

export default defineConfig(async () => {
  // Go 后端监听口：与 alexgo-server/configs/config.yaml 的 server.http_addr 对齐
  // （HEAD 已是 :9090；8080 是改端口前的旧值，代理写死它会让全部 /api 502）。
  // API_PROXY 可覆盖（如反代到远端）。
  const apiTarget = process.env.API_PROXY || 'http://localhost:9090';
  return {
    application: {},
    vite: {
      server: {
        proxy: {
          // 会员端独立代理（MEMBER_PROXY 可覆盖，缺省随主后端——mono 模式 member
          // 路由就挂在同进程；micro 模式才需要指到独立 member-server）。
          // 长前缀必须排在 /api 之前。
          '/api/app/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || apiTarget,
          },
          '/api/admin/member': {
            changeOrigin: true,
            target: process.env.MEMBER_PROXY || apiTarget,
          },
          // 业务 API → Go 后端。不 rewrite：vben baseURL=/api，请求已是 /api/**，gin 挂的正是 /api/**
          '/api': {
            changeOrigin: true,
            target: apiTarget,
          },
          '/swagger': {
            changeOrigin: true,
            target: apiTarget,
          },
        },
      },
    },
  };
});
