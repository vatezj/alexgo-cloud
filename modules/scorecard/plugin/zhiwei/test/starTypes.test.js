import { test } from 'node:test';
import assert from 'node:assert/strict';
import { mkdtempSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { loadStarTypes, LUCKY_STARS, MALEFIC_STARS } from '../src/constants/starTypes.js';

test('初期覆盖六吉、六煞、禄存、天马、四化', () => {
  for (const s of ['左辅', '右弼', '天魁', '天钺', '文昌', '文曲', '禄存', '天马', '化禄', '化权', '化科']) {
    assert.ok(LUCKY_STARS.has(s), `吉星表应含 ${s}`);
  }
  for (const s of ['擎羊', '陀罗', '火星', '铃星', '地空', '地劫', '化忌']) {
    assert.ok(MALEFIC_STARS.has(s), `煞星表应含 ${s}`);
  }
});

test('loadStarTypes 从指定 JSON 文件读取分类表（外置可迭代）', () => {
  const dir = mkdtempSync(join(tmpdir(), 'star-types-'));
  try {
    const file = join(dir, 'custom.json');
    writeFileSync(
      file,
      JSON.stringify({ luckyStars: ['天魁'], maleficStars: ['地空'] }),
    );
    const t = loadStarTypes(file);
    assert.deepEqual([...t.lucky], ['天魁']);
    assert.deepEqual([...t.malefic], ['地空']);
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test('默认分类表与内置 star-types.json 文件内容一致', async () => {
  const { readFileSync } = await import('node:fs');
  const raw = JSON.parse(
    readFileSync(new URL('../src/constants/star-types.json', import.meta.url), 'utf8'),
  );
  assert.deepEqual([...LUCKY_STARS].sort(), [...raw.luckyStars].sort());
  assert.deepEqual([...MALEFIC_STARS].sort(), [...raw.maleficStars].sort());
});
