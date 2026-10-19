import { describe, expect, it } from 'vitest';

import { blobErrorDetail } from './blob-error';

// RF④：download 的 400 错误体是 Blob——必须能解出信封 msg（export 30013 弹
// 「导出行数超过上限」而非乱码/通用错）。
describe('blobErrorDetail', () => {
  it('信封 JSON blob → 提取 msg', async () => {
    const blob = new Blob([
      JSON.stringify({ code: 30013, msg: '导出行数超过上限', data: null }),
    ]);
    await expect(blobErrorDetail(blob)).resolves.toBe('导出行数超过上限');
  });

  it('缺 msg → 回退 message 字段', async () => {
    const blob = new Blob([JSON.stringify({ code: 50000, message: '系统繁忙' })]);
    await expect(blobErrorDetail(blob)).resolves.toBe('系统繁忙');
  });

  it('非 JSON blob → 空串（调用方回退通用文案）', async () => {
    await expect(blobErrorDetail(new Blob(['not json']))).resolves.toBe('');
  });

  it('空对象 → 空串', async () => {
    await expect(blobErrorDetail(new Blob(['{}']))).resolves.toBe('');
  });
});
