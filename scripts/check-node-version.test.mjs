// 票11：根 pnpm lint 在 Node <22.18 上死于 oxlint .ts 配置加载（ERR_UNKNOWN_FILE_EXTENSION），
// 报错晦涩。守卫脚本须按 package.json engines.node 快速失败并给出升级指引。
import assert from 'node:assert/strict';
import { test } from 'node:test';

import { checkNodeVersion } from './check-node-version.mjs';

const ENGINES = '^22.18.0 || ^24.0.0'; // 与 package.json engines.node 同款格式

test('低于 engines 下限的版本必须失败（本机 22.14 场景）', () => {
  const r = checkNodeVersion('22.14.0', ENGINES);
  assert.equal(r.ok, false);
  assert.match(r.error, /22\.14\.0/);
  assert.match(r.error, /22\.18/); // 指引含目标版本
});

test('恰等于下限通过', () => {
  assert.equal(checkNodeVersion('22.18.0', ENGINES).ok, true);
});

test('另一 caret 分支 24.x 通过，25.x 不通过', () => {
  assert.equal(checkNodeVersion('24.1.0', ENGINES).ok, true);
  assert.equal(checkNodeVersion('25.0.0', ENGINES).ok, false);
});

test('中间主版本 23.x 不通过（engines 未声明）', () => {
  assert.equal(checkNodeVersion('23.5.0', ENGINES).ok, false);
});

test('engines 格式不识别时不拦（fail-open，避免误伤 lint）', () => {
  const r = checkNodeVersion('22.14.0', '>=22 <24 || *');
  assert.equal(r.ok, true);
});
