import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';

const DEFAULT_FILE = fileURLToPath(new URL('./star-types.json', import.meta.url));

/**
 * 加载吉星/煞星分类表（产品核心配置，外置 JSON 便于迭代）。
 * @param {string} [file] JSON 文件路径，默认用内置 star-types.json
 * @returns {{lucky:Set<string>, malefic:Set<string>}}
 */
export function loadStarTypes(file = DEFAULT_FILE) {
  const raw = JSON.parse(readFileSync(file, 'utf8'));
  return {
    lucky: new Set(raw.luckyStars ?? []),
    malefic: new Set(raw.maleficStars ?? []),
  };
}

const defaults = loadStarTypes();

/** 六吉星 + 禄存/天马/四化吉化 */
export const LUCKY_STARS = defaults.lucky;

/** 六煞星 + 化忌 */
export const MALEFIC_STARS = defaults.malefic;
