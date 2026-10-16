// 生成代码，勿手改（codegen M4 · codegen_demo_items）。
import { requestClient } from '#/api/request';

import type { CodegenDemoItem } from './types';

/** 查询操作表：键 = JSONName，值 = query_operation 配置（eq/like/between）。 */
export const queryOps: Record<string, string> = {
  id: 'eq',
  name: 'like',
  price: 'like',
  status: 'eq',
  remark: 'like',
  created_at: 'between',
};

/** 客户端过滤：controller 一次性返回全量，查询表单在此过滤。
 * 空值（undefined/null/''）跳过；eq 精确匹配（String 双向），
 * like/between 走子串匹配且不区分大小写（between≈like，见 docs/usage.md）。 */
export function matchQuery<T extends object>(
  rows: T[],
  formValues: Record<string, any>,
): T[] {
  const entries = Object.entries(formValues ?? {}).filter(
    ([, v]) => v !== undefined && v !== null && v !== '',
  );
  if (entries.length === 0) {
    return rows;
  }
  return rows.filter((row) =>
    entries.every(([key, value]) => {
      const cell = String((row as any)[key] ?? '');
      const target = String(value);
      if ((queryOps[key] ?? 'eq') === 'eq') {
        return cell === target;
      }
      return cell.toLowerCase().includes(target.toLowerCase());
    }),
  );
}

/** 列表：一次性返回全量，页面用 localPage 本地切页。 */
export function listCodegenDemoItem(): Promise<CodegenDemoItem[]> {
  return requestClient.get<CodegenDemoItem[]>('/admin/order/codegen_demo_items');
}

// 动作类信封 data:null → 拦截器解包后 null：返回类型 Promise<void>（勿声明实体）。
export function createCodegenDemoItem(
  data: Partial<CodegenDemoItem>,
): Promise<void> {
  return requestClient.post('/admin/order/codegen_demo_items', data);
}

export function updateCodegenDemoItem(
  id: number | string,
  data: Partial<CodegenDemoItem>,
): Promise<void> {
  return requestClient.put(`/admin/order/codegen_demo_items/${id}`, data);
}

export function deleteCodegenDemoItem(id: number | string): Promise<void> {
  return requestClient.delete(`/admin/order/codegen_demo_items/${id}`);
}
