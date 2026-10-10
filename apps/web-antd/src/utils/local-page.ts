/** 旧接口一次性返回全量数组，由前端切页。 */
export function localPage<T>(
  rows: T[],
  currentPage: number,
  pageSize: number,
): T[] {
  const start = (currentPage - 1) * pageSize;
  return rows.slice(start, start + pageSize);
}
