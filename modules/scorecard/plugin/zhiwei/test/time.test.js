import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parsePlayTime } from '../src/utils/time.js';

test('解析 +08:00 偏移的 ISO 时间，按偏移量取年月日时分', () => {
  const r = parsePlayTime('2026-11-05T20:30:00+08:00');
  assert.deepEqual(r, {
    year: 2026,
    month: 11,
    day: 5,
    hour: 20,
    minute: 30,
    utcOffsetMinutes: 480,
  });
});

test('解析 Z 结尾的 UTC 时间', () => {
  const r = parsePlayTime('2026-01-01T00:00:00Z');
  assert.equal(r.utcOffsetMinutes, 0);
  assert.equal(r.hour, 0);
});

test('解析负偏移时区', () => {
  const r = parsePlayTime('2026-06-15T14:05:00-05:00');
  assert.deepEqual(r, {
    year: 2026,
    month: 6,
    day: 15,
    hour: 14,
    minute: 5,
    utcOffsetMinutes: -300,
  });
});

test('缺分钟时按 0 处理', () => {
  const r = parsePlayTime('2026-11-05T20:30+08:00');
  assert.equal(r.minute, 30);
  assert.equal(r.hour, 20);
});

test('非法格式抛错', () => {
  assert.throws(() => parsePlayTime('not-a-date'), /Invalid playTime/);
  assert.throws(() => parsePlayTime('2026-11-05'), /Invalid playTime/);
  assert.throws(() => parsePlayTime(undefined), /Invalid playTime/);
});
