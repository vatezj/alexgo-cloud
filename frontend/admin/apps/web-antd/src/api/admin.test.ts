import { beforeEach, describe, expect, it, vi } from 'vitest';

import { exportOrders, exportUsers } from './admin';
import { requestClient } from './request';

vi.mock('./request', () => ({
  requestClient: {
    delete: vi.fn(),
    download: vi.fn().mockResolvedValue(new Blob()),
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
  },
}));

// P1-2 传输钉：export 走 requestClient.download（默认 GET + responseType
// blob + responseReturn body → Blob），超时放宽 60s（同 codegen 下载）。
describe('export transport contract', () => {
  beforeEach(() => {
    vi.mocked(requestClient.download).mockClear();
  });

  it('exportUsers 走 download，路径与超时正确', async () => {
    await exportUsers();
    expect(requestClient.download).toHaveBeenCalledWith(
      '/admin/system/users/export',
      expect.objectContaining({ timeout: 60_000 }),
    );
  });

  it('exportOrders 走 download，路径与超时正确', async () => {
    await exportOrders();
    expect(requestClient.download).toHaveBeenCalledWith(
      '/admin/order/orders/export',
      expect.objectContaining({ timeout: 60_000 }),
    );
  });
});
