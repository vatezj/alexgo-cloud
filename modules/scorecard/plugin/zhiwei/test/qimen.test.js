import { test } from 'node:test';
import assert from 'node:assert/strict';
import { getQimenPalaces, qimenCache } from '../src/services/qimen.js';

const PLAY_TIME = '2026-11-05T20:30:00+08:00';

test('返回九宫适配结构：palaceIndex/palaceName/door/star/deity 为中文', () => {
  const palaces = getQimenPalaces({ playTime: PLAY_TIME });
  assert.equal(palaces.length, 9);

  for (const p of palaces) {
    assert.ok(p.palaceIndex >= 1 && p.palaceIndex <= 9);
    assert.match(p.palaceName, /^[坎坤震巽中乾兑艮离]宫$|^中五宫$/);
    // 中五宫无门无神
    if (p.palaceIndex === 5) {
      assert.equal(p.door, null);
      assert.equal(p.deity, null);
    } else {
      assert.match(p.door, /^[休死伤杜开惊生景]门$/);
      assert.match(p.deity, /^(值符|腾蛇|太阴|六合|白虎|玄武|九地|九天)$/);
    }
  }
});

test('生门必在且仅在一个非中宫，落宫方向可映射', () => {
  const palaces = getQimenPalaces({ playTime: PLAY_TIME });
  const shengmen = palaces.filter((p) => p.door === '生门');
  assert.equal(shengmen.length, 1, `生门数量应为1，实际${shengmen.length}`);
  assert.notEqual(shengmen[0].palaceIndex, 5, '生门不落中五宫');
});

test('同一时间两次调用结果一致（确定性）', () => {
  const a = getQimenPalaces({ playTime: PLAY_TIME });
  const b = getQimenPalaces({ playTime: PLAY_TIME });
  assert.deepEqual(a, b);
});

test('按字符串自身的钟表时间起局（同一钟表时间同盘，不同钟表时间不同盘）', () => {
  // 奇门按本地钟表时间起局：20:30+08:00 与 20:30+00:00 钟表时间相同 → 同盘
  const a = getQimenPalaces({ playTime: '2026-11-05T20:30:00+08:00' });
  const b = getQimenPalaces({ playTime: '2026-11-05T20:30:00+00:00' });
  assert.deepEqual(a, b, '相同钟表时间应同盘');

  // 08:30 与 20:30 钟表时间不同 → 不同盘（证明取的是字符串时刻而非换算后时刻）
  const c = getQimenPalaces({ playTime: '2026-11-05T08:30:00+08:00' });
  assert.notDeepEqual(a, c, '不同时刻应不同盘');
});

test('非法 playTime 抛错', () => {
  assert.throws(() => getQimenPalaces({ playTime: 'garbage' }));
  assert.throws(() => getQimenPalaces({}));
});

test('同一小时内的请求命中缓存（返回同一引用）', () => {
  qimenCache.clear();
  const a = getQimenPalaces({ playTime: '2026-11-05T20:10:00+08:00' });
  assert.equal(qimenCache.size, 1);
  const b = getQimenPalaces({ playTime: '2026-11-05T20:50:00+08:00' });
  assert.equal(qimenCache.size, 1, '同小时不应新增缓存条目');
  assert.equal(a, b, '命中应返回同一引用');
});

test('不同小时各自成条目（含 23 点换日前后不串盘）', () => {
  qimenCache.clear();
  getQimenPalaces({ playTime: '2026-11-05T00:10:00+08:00' });
  getQimenPalaces({ playTime: '2026-11-05T23:10:00+08:00' });
  getQimenPalaces({ playTime: '2026-11-05T20:10:00+08:00' });
  assert.equal(qimenCache.size, 3, '三个不同小时应有三条缓存');
});

test('缓存超上限时淘汰最旧条目', () => {
  qimenCache.clear();
  const pad = (n) => String(n).padStart(2, '0');
  // 灌入 257 个不同小时的请求
  for (let i = 0; i < 257; i++) {
    const d = new Date(Date.UTC(2026, 0, 1 + Math.floor(i / 24)));
    const hour = i % 24;
    const playTime = `${d.getUTCFullYear()}-${pad(d.getUTCMonth() + 1)}-${pad(d.getUTCDate())}T${pad(hour)}:00:00+08:00`;
    getQimenPalaces({ playTime });
  }
  assert.ok(qimenCache.size <= 256, `缓存应有上限，实际 ${qimenCache.size}`);
});
