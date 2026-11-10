import express from 'express';
import cors from 'cors';
import calcRouter from './routes/calc.js';

/** 构建 Express 应用（与监听解耦，便于测试） */
export function createApp() {
  const app = express();
  app.use(cors());
  app.use(express.json());
  app.use('/api', calcRouter);

  // 统一 JSON 错误响应（含 body 解析失败），避免默认 HTML 错误页
  // eslint-disable-next-line no-unused-vars
  app.use((err, req, res, next) => {
    if (err?.type === 'entity.parse.failed') {
      return res.status(400).json({ error: '请求体 JSON 格式错误' });
    }
    console.error(err);
    res.status(500).json({ error: '服务器内部错误' });
  });

  return app;
}
