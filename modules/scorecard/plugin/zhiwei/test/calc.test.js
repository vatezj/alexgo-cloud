import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { createApp } from '../src/app.js';

let server;
let base;

before(async () => {
  const app = createApp();
  await new Promise((resolve) => {
    server = app.listen(0, resolve);
  });
  base = `http://127.0.0.1:${server.address().port}`;
});

after(() => server.close());

const VALID_BODY = {
  birthDate: '2000-08-16',
  birthHourIndex: 2,
  gender: 'female',
  playTime: '2026-11-05T20:30:00+08:00',
};

test('POST /api/calc 返回推荐列表 + meta + disclaimer', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(VALID_BODY),
  });
  assert.equal(res.status, 200);
  const data = await res.json();

  assert.ok(Array.isArray(data.recommendations));
  assert.ok(data.recommendations.length >= 1, '至少一条推荐');
  assert.ok(data.recommendations.length <= 3);
  const first = data.recommendations[0];
  assert.equal(first.rank, 1);
  assert.ok(typeof first.direction === 'string');
  assert.ok(typeof first.score === 'number' && first.score > 0);
  assert.ok(typeof first.reason === 'string' && first.reason.length > 0);
  // rank 连续递增
  data.recommendations.forEach((r, i) => assert.equal(r.rank, i + 1));

  assert.equal(data.meta.qimenConvention, '时家奇门·拆补法·转盘·中五寄坤');
  assert.equal(data.meta.dayBoundary, '23:00子初换日');
  assert.equal(data.meta.trueSolarTime, false);
  assert.match(data.disclaimer, /仅供娱乐参考/);
});

test('缺少必要参数返回 400', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ birthDate: '2000-08-16' }),
  });
  assert.equal(res.status, 400);
  const data = await res.json();
  assert.ok(data.error);
});

test('非法时间格式返回 500 且带错误信息', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...VALID_BODY, playTime: 'bad-time' }),
  });
  assert.equal(res.status, 500);
  const data = await res.json();
  assert.ok(data.error);
});

test('非法 birthHourIndex 返回 400', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...VALID_BODY, birthHourIndex: 99 }),
  });
  assert.equal(res.status, 400);
});

test('非法 gender 返回 400', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ...VALID_BODY, gender: 'robot' }),
  });
  assert.equal(res.status, 400);
});

test('畸形 JSON body 返回 JSON 格式的 400', async () => {
  const res = await fetch(`${base}/api/calc`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: '{bad json',
  });
  assert.equal(res.status, 400);
  const data = await res.json();
  assert.ok(data.error, '应返回 JSON 错误体而非 HTML 堆栈');
});
