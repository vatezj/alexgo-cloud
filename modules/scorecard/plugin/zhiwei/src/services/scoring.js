import {
  QIMEN_PALACE_TO_DIRECTION,
  ZIWEI_BRANCH_TO_DIRECTION,
} from '../constants/mappings.js';
import { LUCKY_STARS, MALEFIC_STARS } from '../constants/starTypes.js';

/** 生门在九宫的旺衰权重：艮坤旺、离相、乾兑休、坎囚、震巽死 */
const SHENGMEN_WANG_SHUAI = {
  艮宫: 15,
  坤宫: 15,
  离宫: 8,
  乾宫: 0,
  兑宫: 0,
  坎宫: -5,
  震宫: -10,
  巽宫: -10,
};

/** 紫微计分目标宫 */
const TARGET_PALACES = new Set(['命宫', '财帛宫', '迁移宫']);

/**
 * 评分引擎（纯函数）：输入奇门/紫微结构化数据，输出排序后的推荐方位。
 * @param {{palaceIndex:number, door:string, palaceName?:string}[]} qimenPalaces
 * @param {{name:string, earthlyBranch:string, stars:{name:string}[]}[]} ziweiPalaces
 */
export function scoreSeats(qimenPalaces, ziweiPalaces) {
  const scores = {};
  for (const dir of Object.values(QIMEN_PALACE_TO_DIRECTION)) {
    scores[dir] = { score: 0, reasons: [] };
  }

  // ---- 奇门维度：生门落方 +50，再按宫名旺衰加权 ----
  const shengmen = qimenPalaces.find((p) => p.door === '生门');
  if (shengmen) {
    const dir = QIMEN_PALACE_TO_DIRECTION[shengmen.palaceIndex];
    if (dir && scores[dir]) {
      scores[dir].score += 50;
      // 上架口径：reason 仅中性评分说明（fun-compass-review-copy D2），分值逻辑不动
      scores[dir].reasons.push('传统方位模型在此向给出正向评分');

      const palaceName = shengmen.palaceName;
      const wangShuai = palaceName ? SHENGMEN_WANG_SHUAI[palaceName] : undefined;
      if (wangShuai !== undefined) {
        scores[dir].score += wangShuai;
        if (wangShuai > 0) scores[dir].reasons.push('该向综合权重上调');
        if (wangShuai < 0) scores[dir].reasons.push('该向综合权重下调');
      }
    }
  }

  // ---- 紫微维度：目标宫吉星聚 +25 / 煞星聚 -15 ----
  for (const zp of ziweiPalaces) {
    const dir = ZIWEI_BRANCH_TO_DIRECTION[zp.earthlyBranch];
    if (!dir || !scores[dir] || !TARGET_PALACES.has(zp.name)) continue;

    const lucky = zp.stars.filter((s) => LUCKY_STARS.has(s.name));
    const malefic = zp.stars.filter((s) => MALEFIC_STARS.has(s.name));

    if (lucky.length >= 2) {
      scores[dir].score += 25;
      scores[dir].reasons.push('辅助因子聚集，得分上调');
    }
    if (malefic.length >= 2) {
      scores[dir].score -= 15;
      scores[dir].reasons.push('辅助因子不利，得分下调，建议优先其他向');
    }
  }

  return Object.entries(scores)
    .map(([direction, v]) => ({
      direction,
      score: v.score,
      reason: v.reasons.length ? v.reasons.join('；') : '无显著方位信号',
    }))
    .filter((r) => r.score > 0) // 只返回正分方位，避免推荐“煞方”
    .sort((a, b) => b.score - a.score)
    .slice(0, 3);
}
