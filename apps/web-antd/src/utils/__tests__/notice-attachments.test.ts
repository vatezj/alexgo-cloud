import type { NoticeAttachment } from '#/utils/notice-attachments';

import {
  attachmentsFromUploadList,
  parseAttachments,
  stringifyAttachments,
  uploadNoticeAttachment,
} from '#/utils/notice-attachments';
import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('#/api/infra', () => ({
  uploadFile: vi.fn(),
}));

import { uploadFile } from '#/api/infra';

describe('notice-attachments', () => {
  beforeEach(() => {
    vi.mocked(uploadFile).mockReset();
  });

  it('parseAttachments 解析合法 JSON 数组', () => {
    const raw = JSON.stringify([
      { id: 1, name: 'a.jpg', size: 4, url: '/api/public/infra/file/k1' },
    ]);
    expect(parseAttachments(raw)).toHaveLength(1);
    expect(parseAttachments(raw)[0]?.name).toBe('a.jpg');
  });

  it('parseAttachments 对空串/非法 JSON/非数组返回 []（老数据无附件不炸列表）', () => {
    expect(parseAttachments(undefined)).toEqual([]);
    expect(parseAttachments(null)).toEqual([]);
    expect(parseAttachments('')).toEqual([]);
    expect(parseAttachments('not-json')).toEqual([]);
    expect(parseAttachments('{"a":1}')).toEqual([]);
    expect(parseAttachments('[1,2]')).toEqual([]); // 元素非对象也不炸
  });

  it('stringifyAttachments → parseAttachments 往返恒等', () => {
    const list: NoticeAttachment[] = [
      { id: 2, name: 'b.pdf', size: 9, url: '/api/public/infra/file/k2' },
    ];
    expect(parseAttachments(stringifyAttachments(list))).toEqual(list);
    expect(stringifyAttachments([])).toBe('[]');
  });

  it('attachmentsFromUploadList 只收 done 且有 response 的项', () => {
    const list = [
      { status: 'done', response: { id: 1, name: 'a.jpg', size: 4, public_url: '/api/public/infra/file/k1' } },
      { status: 'uploading' },
      { status: 'done' }, // 无 response
      { status: 'error' },
    ];
    expect(attachmentsFromUploadList(list as any)).toEqual([
      { id: 1, name: 'a.jpg', size: 4, url: '/api/public/infra/file/k1' },
    ]);
    expect(attachmentsFromUploadList(undefined)).toEqual([]);
  });

  // Review Focus ⑤：公告附件必须恒 public:true——private 行的 public_url
  // 在 /api/public 下恒 404（30010），公告一发附件即死。
  it('uploadNoticeAttachment 以 public:true + bizType:notice 上传并映射 url', async () => {
    vi.mocked(uploadFile).mockResolvedValue({
      id: 7,
      name: 'c.jpg',
      size: 5,
      mime: 'image/jpeg',
      public_url: '/api/public/infra/file/kk7',
    });
    const file = new File(['x'], 'c.jpg', { type: 'image/jpeg' });
    const att = await uploadNoticeAttachment(file);
    expect(uploadFile).toHaveBeenCalledWith(file, {
      bizType: 'notice',
      public: true,
    });
    expect(att).toEqual({
      id: 7,
      name: 'c.jpg',
      size: 5,
      url: '/api/public/infra/file/kk7',
    });
  });
});
