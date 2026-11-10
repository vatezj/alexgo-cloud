import { astro } from 'iztro';

/**
 * 紫微排盘封装：适配 iztro 返回结构为评分引擎所需的统一格式。
 *
 * 适配点：
 * - iztro 宫名不带“宫”后缀（如“财帛”），统一补齐为“财帛宫”
 * - 四化是星曜的 mutagen 属性（如武曲化权），展开为独立的“化权”星名供评分匹配
 *
 * @param {{birthDate:string, birthHourIndex:number, gender:string}} input
 *   birthDate: 'YYYY-MM-DD'；birthHourIndex: 0–12（0=早子，12=晚子）；gender: 'male'|'female'
 * @returns {{name:string, earthlyBranch:string, stars:{name:string, brightness:string}[]}[]}
 */
export function getZiweiPalaces({ birthDate, birthHourIndex, gender }) {
  const astrolabe = astro.bySolar(
    birthDate,
    birthHourIndex,
    gender === 'female' ? '女' : '男',
    true, // 闰月修正
    'zh-CN',
  );

  return astrolabe.palaces.map((p) => {
    const rawStars = p.majorStars.concat(p.minorStars);
    const stars = [];
    for (const s of rawStars) {
      stars.push({ name: s.name, brightness: s.brightness ?? '' });
      if (s.mutagen) {
        // 四化展开为独立星名（化禄/化权/化科/化忌），评分引擎按名匹配
        stars.push({ name: `化${s.mutagen}`, brightness: s.brightness ?? '' });
      }
    }
    const name = p.name.endsWith('宫') ? p.name : `${p.name}宫`;
    return { name, earthlyBranch: p.earthlyBranch, stars };
  });
}
