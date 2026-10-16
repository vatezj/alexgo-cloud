import { requestClient } from './request';
import { uploadFile } from './infra';

import { describe, expect, it, vi } from 'vitest';

// 只桩传输层：pin uploadFile → requestClient.post 的调用形状（真实网络不测）。
vi.mock('./request', () => ({
  requestClient: { post: vi.fn().mockResolvedValue({}) },
}));

describe('uploadFile transport contract', () => {
  // 回归钉：RequestClient 默认 CT 是 application/json——裸 post(fd) 会让 axios
  // transformRequest 把 FormData JSON.stringify 成 {"file":{},...}（body 非
  // multipart → 后端 ctx.FormFile 恒 4xx）。必须显式 multipart CT 走
  // 「非 JSON CT 跳过 stringify」分支（axios defaults/index.js:55-57）。
  it('post 第三参携带 multipart/form-data Content-Type 且 data 为 FormData', async () => {
    const file = new File(['x'], 'a.jpg', { type: 'image/jpeg' });
    await uploadFile(file, { bizType: 'notice', public: true });

    expect(requestClient.post).toHaveBeenCalledTimes(1);
    expect(requestClient.post).toHaveBeenCalledWith(
      '/admin/infra/file/upload',
      expect.any(FormData),
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'multipart/form-data',
        }),
      }),
    );
  });
});
