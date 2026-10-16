import { uploadFile } from '#/api/infra';

/** 公告附件元素（后端原样存取的 JSON 数组元素，spec §2）。 */
export interface NoticeAttachment {
  id: number;
  name: string;
  size: number;
  url: string; // /api/public/infra/file/<key>
}

/** antd UploadFile 的最小鸭子形状（customRequest onSuccess 后 response=FileMeta）。 */
interface UploadItemLike {
  response?: { id?: number; name?: string; size?: number; public_url?: string };
  status?: string;
}

/** 解析附件 JSON 文本；空/非法/非数组一律 []（老公告无此字段不炸列表）。 */
export function parseAttachments(raw?: null | string): NoticeAttachment[] {
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed.filter(
      (x): x is NoticeAttachment =>
        !!x &&
        typeof x === 'object' &&
        typeof x.id === 'number' &&
        typeof x.name === 'string' &&
        typeof x.url === 'string',
    );
  } catch {
    return [];
  }
}

/** 序列化为后端原样存储的 JSON 文本。 */
export function stringifyAttachments(list: NoticeAttachment[]): string {
  return JSON.stringify(list ?? []);
}

/** Upload fileList → 附件数组：只收 done 且携带 response（= 上传成功回包）的项。 */
export function attachmentsFromUploadList(
  list?: null | UploadItemLike[],
): NoticeAttachment[] {
  if (!Array.isArray(list)) return [];
  const out: NoticeAttachment[] = [];
  for (const f of list) {
    const r = f?.response;
    if (f?.status !== 'done' || !r?.public_url) continue;
    out.push({
      id: r.id ?? 0,
      name: r.name ?? '',
      size: r.size ?? 0,
      url: r.public_url,
    });
  }
  return out;
}

/**
 * 公告附件上传：恒 public:true + bizType:notice（Review Focus ⑤）——
 * private 行的 URL 在 /api/public 下恒 404，公告附件必须可跨端直读。
 */
export async function uploadNoticeAttachment(file: File): Promise<NoticeAttachment> {
  const meta = await uploadFile(file, { bizType: 'notice', public: true });
  return { id: meta.id, name: meta.name, size: meta.size, url: meta.public_url };
}
