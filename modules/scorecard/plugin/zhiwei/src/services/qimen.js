import { calculate } from '@spensercai/qimen-rs';
import { parsePlayTime } from '../utils/time.js';
import {
  QIMEN_PALACE_TO_NAME,
  QIMEN_DOOR_TO_CHINESE,
  QIMEN_STAR_TO_CHINESE,
  QIMEN_DEITY_TO_CHINESE,
} from '../constants/mappings.js';

/** 缓存上限：超过后淘汰最早插入的条目 */
const MAX_CACHE = 256;

/** 时辰级结果缓存（key 见 cacheKey） */
export const qimenCache = new Map();

function cacheKey({ year, month, day, hour, utcOffsetMinutes }) {
  // 按小时缓存（比文档的时辰级更细）；offset 影响节气在本地民用时间的落点，故入 key。
  // 注意：节气恰好落在某小时内时，该小时内的两次请求可能得到不同盘——
  // 接受此边界（娱乐场景，文档亦按 2 小时时辰窗口缓存）。
  return `${year}-${month}-${day}-${hour}|${utcOffsetMinutes}`;
}

/**
 * 奇门起局封装：将 qimen-rs 原始返回适配为内部统一结构，
 * 避免库的英文枚举/字段命名渗透进评分逻辑。
 *
 * qimen-rs 语义：输入为指定 utc_offset 下的民用钟表时间，不读取机器时区；
 * 默认 23:00 子初换日、时家奇门·拆补法·转盘·中五寄坤。
 *
 * @param {{playTime:string}} input playTime 为带时区偏移的 ISO 8601 字符串
 * @returns {{palaceIndex:number, palaceName:string, door:string|null, star:string, deity:string|null}[]}
 */
export function getQimenPalaces({ playTime }) {
  const t = parsePlayTime(playTime);
  const key = cacheKey(t);
  const cached = qimenCache.get(key);
  if (cached) return cached;

  const result = calculate({
    year: t.year,
    month: t.month,
    day: t.day,
    hour: t.hour,
    minute: t.minute,
    utc_offset_minutes: t.utcOffsetMinutes,
  });

  const palaces = result.palaces.map((p) => ({
    palaceIndex: p.number,
    palaceName: QIMEN_PALACE_TO_NAME[p.number],
    door: p.door ? (QIMEN_DOOR_TO_CHINESE[p.door] ?? p.door) : null,
    star: (p.stars ?? [])
      .map((s) => QIMEN_STAR_TO_CHINESE[s] ?? s)
      .join('、'),
    deity: p.deity ? (QIMEN_DEITY_TO_CHINESE[p.deity] ?? p.deity) : null,
  }));

  if (qimenCache.size >= MAX_CACHE) {
    // Map 迭代序 = 插入序，删最旧
    qimenCache.delete(qimenCache.keys().next().value);
  }
  qimenCache.set(key, palaces);
  return palaces;
}
