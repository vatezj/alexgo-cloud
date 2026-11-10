/**
 * 时间归一化：解析带时区偏移的 ISO 8601 字符串，
 * 按字符串自身的偏移量取出年月日时分，不依赖机器时区。
 *
 * @param {string} playTime 如 '2026-11-05T20:30:00+08:00' / '...Z'
 * @returns {{year:number, month:number, day:number, hour:number, minute:number, utcOffsetMinutes:number}}
 */
export function parsePlayTime(playTime) {
  const m =
    typeof playTime === 'string' &&
    playTime.match(
      /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})(?::(\d{2}))?(Z|[+-]\d{2}:?\d{2})$/,
    );
  if (!m) {
    throw new Error(`Invalid playTime: ${playTime}`);
  }

  const [, y, mo, d, h, mi, s, tz] = m;
  let utcOffsetMinutes;
  if (tz === 'Z') {
    utcOffsetMinutes = 0;
  } else {
    const sign = tz[0] === '-' ? -1 : 1;
    const hh = Number(tz.slice(1, 3));
    const mm = Number(tz.slice(-2));
    utcOffsetMinutes = sign * (hh * 60 + mm);
  }

  return {
    year: Number(y),
    month: Number(mo),
    day: Number(d),
    hour: Number(h),
    minute: Number(mi),
    utcOffsetMinutes,
  };
}
