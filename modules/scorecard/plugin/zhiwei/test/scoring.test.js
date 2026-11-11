import { test } from 'node:test';
import assert from 'node:assert/strict';
import { scoreSeats } from '../src/services/scoring.js';

/** 构造九宫奇门数据：door 为中文门名，palaceName 为宫名 */
function qimenFixture(shengmenPalaceIndex) {
  const names = {
    1: '坎宫', 2: '坤宫', 3: '震宫', 4: '巽宫', 5: '中五宫',
    6: '乾宫', 7: '兑宫', 8: '艮宫', 9: '离宫',
  };
  return Object.keys(names).map((n) => {
    const palaceIndex = Number(n);
    return {
      palaceIndex,
      palaceName: names[n],
      door: palaceIndex === shengmenPalaceIndex ? '生门' : '开门',
      star: '天蓬',
      deity: '值符',
    };
  });
}

/** 构造紫微宫数据 */
function ziweiPalace(name, earthlyBranch, starNames) {
  return {
    name,
    earthlyBranch,
    stars: starNames.map((n) => ({ name: n, brightness: '庙' })),
  };
}

test('生门落艮宫：+50 且旺衰 +15，得 65 分并带两条理由', () => {
  const r = scoreSeats(qimenFixture(8), []);
  const east = r.find((x) => x.direction === '东北');
  assert.ok(east, '东北方向应存在');
  assert.equal(east.score, 65);
  assert.match(east.reason, /正向评分/);
  assert.match(east.reason, /综合权重上调/);
});

test('生门落坎宫（囚）：+50-5=45，理由含权重下调', () => {
  const r = scoreSeats(qimenFixture(1), []);
  const north = r.find((x) => x.direction === '北');
  assert.equal(north.score, 45);
  assert.match(north.reason, /综合权重下调/);
});

test('生门落震宫（死）：+50-10=40', () => {
  const r = scoreSeats(qimenFixture(3), []);
  const east = r.find((x) => x.direction === '东');
  assert.equal(east.score, 40);
});

test('无生门时所有方向 0 分，返回空数组', () => {
  const r = scoreSeats(qimenFixture(null), []);
  assert.deepEqual(r, []);
});

test('目标宫吉星≥2：+25 并给出中性上调理由', () => {
  const ziwei = [
    ziweiPalace('财帛宫', '寅', ['禄存', '天马', '擎羊']),
  ];
  const r = scoreSeats(qimenFixture(null), ziwei);
  const east = r.find((x) => x.direction === '东北');
  assert.equal(east.score, 25);
  assert.match(east.reason, /辅助因子聚集/);
  assert.match(east.reason, /得分上调/);
});

test('目标宫煞星≥2：-15 并提示优先其他向', () => {
  const ziwei = [
    ziweiPalace('命宫', '午', ['擎羊', '陀罗', '左辅']),
  ];
  const r = scoreSeats(qimenFixture(null), ziwei);
  // 单独 -15 为负分被过滤；与生门叠加验证
  const withSheng = scoreSeats(qimenFixture(9), ziwei);
  const south = withSheng.find((x) => x.direction === '南');
  assert.equal(south.score, 50 + 8 - 15);
  assert.match(south.reason, /辅助因子不利/);
  assert.equal(r.length, 0, '纯负分方向不应返回');
});

test('非目标宫（夫妻宫）吉星不计分', () => {
  const ziwei = [ziweiPalace('夫妻宫', '辰', ['左辅', '右弼'])];
  const r = scoreSeats(qimenFixture(null), ziwei);
  assert.deepEqual(r, []);
});

test('迁移宫计入目标宫', () => {
  const ziwei = [ziweiPalace('迁移宫', '子', ['文昌', '文曲'])];
  const r = scoreSeats(qimenFixture(null), ziwei);
  const north = r.find((x) => x.direction === '北');
  assert.equal(north.score, 25);
});

test('结果按分数降序、最多 3 条、只含正分', () => {
  // 生门落艮(东北 65) + 财帛宫寅(东北 +25) + 命宫卯(东 +25)
  const ziwei = [
    ziweiPalace('财帛宫', '寅', ['禄存', '天马']),
    ziweiPalace('命宫', '卯', ['左辅', '右弼']),
  ];
  const r = scoreSeats(qimenFixture(8), ziwei);
  assert.ok(r.length <= 3);
  assert.ok(r.every((x) => x.score > 0));
  for (let i = 1; i < r.length; i++) {
    assert.ok(r[i - 1].score >= r[i].score, '应按分数降序');
  }
  // 东北 = 65 + 25 = 90，应排第一；东 = 25 第二
  assert.equal(r[0].direction, '东北');
  assert.equal(r[0].score, 90);
  assert.equal(r[1].direction, '东');
  assert.equal(r[1].score, 25);
});

test('生门落乾宫（休，权重 0）：仅 +50 主理由', () => {
  const r = scoreSeats(qimenFixture(6), []);
  const northwest = r.find((x) => x.direction === '西北');
  assert.equal(northwest.score, 50);
  assert.match(northwest.reason, /正向评分/);
  assert.ok(!/权重上调|权重下调/.test(northwest.reason), '权重 0 不应有旺衰理由');
});
