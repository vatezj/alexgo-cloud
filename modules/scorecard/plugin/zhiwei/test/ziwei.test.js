import { test } from 'node:test';
import assert from 'node:assert/strict';
import { getZiweiPalaces } from '../src/services/ziwei.js';

const INPUT = { birthDate: '2000-08-16', birthHourIndex: 2, gender: 'female' };

test('返回 12 宫，含宫名/地支/星曜', () => {
  const palaces = getZiweiPalaces(INPUT);
  assert.equal(palaces.length, 12);
  for (const p of palaces) {
    assert.ok(typeof p.name === 'string' && p.name.length > 0, '宫名应非空');
    assert.ok(typeof p.earthlyBranch === 'string', '地支应为字符串');
    assert.ok(Array.isArray(p.stars), 'stars 应为数组');
  }
});

test('宫名统一为带“宫”后缀（适配 iztro 的“财帛”→“财帛宫”）', () => {
  const palaces = getZiweiPalaces(INPUT);
  const names = palaces.map((p) => p.name);
  assert.ok(names.includes('命宫'));
  assert.ok(names.includes('财帛宫'), `实际宫名: ${names.join(',')}`);
  assert.ok(names.includes('迁移宫'));
  assert.ok(names.every((n) => n.endsWith('宫')), `全部应带宫后缀: ${names.join(',')}`);
});

test('地支取值在十二地支内', () => {
  const branches = new Set(
    '子丑寅卯辰巳午未申酉戌亥'.split(''),
  );
  const palaces = getZiweiPalaces(INPUT);
  for (const p of palaces) {
    assert.ok(branches.has(p.earthlyBranch), `非法地支: ${p.earthlyBranch}`);
  }
});

test('male/female 均可排盘，各返回 12 宫', () => {
  // 性别只影响大限顺逆，不改变宫位结构；此处验证两种入参都能正常排盘
  const female = getZiweiPalaces(INPUT);
  const male = getZiweiPalaces({ ...INPUT, gender: 'male' });
  assert.equal(female.length, 12);
  assert.equal(male.length, 12);
});

test('四化以化禄/化权/化科/化忌 星名暴露（供评分匹配）', () => {
  // 该出生盘含武曲化权等四化，星列表中应出现化* 名
  const palaces = getZiweiPalaces(INPUT);
  const allStars = palaces.flatMap((p) => p.stars.map((s) => s.name));
  const mutagens = allStars.filter((n) => /^化(禄|权|科|忌)$/.test(n));
  assert.ok(mutagens.length > 0, `应至少有一个四化星，实际: ${allStars.join(',')}`);
});
