// 票11：lint 前 Node 版本守卫。oxlint/oxfmt 的 .ts 配置需 Node ^20.19 || >=22.18
// 原生 type-strip，低版本死于 ERR_UNKNOWN_FILE_EXTENSION——报错晦涩。此处按
// package.json engines.node（caret 子集）先判，失败即给出升级指引并非零退出。
// engines 含不识别的 range 语法时 fail-open（不拦），避免误伤。
import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

/**
 * checkNodeVersion 按 caret-or 范围判版本。
 * @param {string} version 点分版本（如 "22.14.0"）
 * @param {string} engines 仅支持 "^x.y.z || ^a.b.c" 形式
 * @returns {{ok: boolean, error?: string}}
 */
export function checkNodeVersion(version, engines) {
  const clauses = engines.split('||').map((s) => s.trim());
  const parsed = clauses.map((c) => {
    const m = /^\^(\d+)\.(\d+)\.(\d+)$/.exec(c);
    return m ? [Number(m[1]), Number(m[2]), Number(m[3])] : null;
  });
  if (parsed.some((p) => p === null)) {
    return { ok: true }; // range 语法超集，fail-open 交给 pnpm 自己的 engine 检查
  }
  const [maj, min, pat] = version.split('.').map(Number);
  const ok = parsed.some(([M, m, p]) => maj === M && (min > m || (min === m && pat >= p)));
  if (ok) {
    return { ok: true };
  }
  return {
    error:
      `Node.js ${version} 不满足 engines.node "${engines}"。` +
      `lint（oxlint/oxfmt 的 .ts 配置）需 ^20.19 或 >=22.18；` +
      `请升级 Node（如 nvm install 22.18 / 24）后重试。`,
    ok: false,
  };
}

// 直接执行时读真实版本并守卫（被 import 时不触发，供单测）。
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const pkgPath = join(dirname(fileURLToPath(import.meta.url)), '..', 'package.json');
  const { engines } = JSON.parse(readFileSync(pkgPath, 'utf8'));
  const r = checkNodeVersion(process.versions.node, engines?.node ?? '');
  if (!r.ok) {
    console.error(`\n✖ ${r.error}\n`);
    process.exit(1);
  }
}
