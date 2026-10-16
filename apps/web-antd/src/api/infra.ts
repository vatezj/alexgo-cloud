import { downloadFileFromBlob } from '@vben/utils';

import { requestClient } from './request';

// ---- 代码生成（infra codegen，/admin/infra/codegen/*）----
// 列表走统一分页信封 data:{list,total,page,size}（resp.Page，size 上限 100），
// 由 requestClient 拦截器解包——到这里是 {list,total} 裸对象。

export interface CodegenTable {
  id: number;
  table_name: string;
  table_comment: string;
  module: string;
  business_name: string;
  class_name: string;
  template_type: number; // 1 单表 2 树 3 主子
  front_type: number; // 1 vben5 antd
  parent_table_id: number;
  remark: string;
  created_at: string;
  updated_at: string;
}

// 快照（name/type/comment/go_type/json_name/主键空性）+ 用户配置（html_type/
// 开关/query_operation/dict_type/sort_order），同步不覆盖配置（spec §9.4）。
export interface CodegenColumn {
  id: number;
  table_id: number;
  name: string;
  type: string;
  comment: string;
  go_type: string;
  json_name: string;
  is_pk: boolean;
  auto_increment: boolean;
  nullable: boolean;
  html_type: string;
  list_enable: boolean;
  form_enable: boolean;
  query_enable: boolean;
  query_operation: string; // eq / like / between
  list_required: boolean;
  form_required: boolean;
  dict_type: string;
  sort_order: number;
  deprecated: boolean;
}

export interface SyncResult {
  added: number;
  updated: number;
  deprecated: number;
  unchanged: number;
}

// 列表页服务端真分页（resp.Page 信封，PageReq size 上限 100）。
export function listCodegenTablesPage(params: { page: number; size: number }) {
  return requestClient.get<{ list: CodegenTable[]; total: number }>(
    '/admin/infra/codegen/tables',
    { params },
  );
}

// 「全部表」预览拉全：按服务端上限 100/页循环分页，杜绝大批量表时
// 「生成代码」静默截断。判停：末批 < 100；安全上限 100 页（1 万张），
// 防后端异常时死循环。
export async function listAllCodegenTables(): Promise<CodegenTable[]> {
  const pageSize = 100;
  const maxPages = 100;
  const all: CodegenTable[] = [];
  for (let page = 1; page <= maxPages; page += 1) {
    const { list } = await listCodegenTablesPage({ page, size: pageSize });
    all.push(...list);
    if (list.length < pageSize) {
      break;
    }
    if (page === maxPages) {
      console.warn(
        `[codegen] 表数量超过 ${pageSize * maxPages} 张，仅加载前 ${pageSize * maxPages} 张`,
      );
    }
  }
  return all;
}

// 导入弹窗下拉：库内全部可导入表名（information_schema 直读）。
export function listDBTables() {
  return requestClient.get<string[]>('/admin/infra/codegen/db-tables');
}

export function importCodegenTable(tableName: string) {
  return requestClient.post<CodegenTable>('/admin/infra/codegen/import', {
    table_name: tableName,
  });
}

export function updateCodegenTable(
  id: number,
  payload: Partial<CodegenTable>,
): Promise<void> {
  return requestClient.put(`/admin/infra/codegen/tables/${id}`, payload);
}

export function deleteCodegenTable(id: number): Promise<void> {
  return requestClient.delete(`/admin/infra/codegen/tables/${id}`);
}

export function listCodegenColumns(id: number) {
  return requestClient.get<CodegenColumn[]>(
    `/admin/infra/codegen/tables/${id}/columns`,
  );
}

// 全量保存：后端按提交数组整体替换该表配置。
export function saveCodegenColumns(
  id: number,
  columns: CodegenColumn[],
): Promise<void> {
  return requestClient.put(
    `/admin/infra/codegen/tables/${id}/columns`,
    columns,
  );
}

export function syncCodegenTable(id: number) {
  return requestClient.post<SyncResult>(
    `/admin/infra/codegen/tables/${id}/sync`,
  );
}

// ---- M3：预览 / ZIP 下载 ----

export interface CodegenFile {
  content: string;
  path: string;
}

// 预览选中表的生成结果（后端纯内存渲染，不落盘）。
// 大批量渲染可能超过默认 10s 超时，显式放宽到 60s。
export function previewCodegen(tableIds: number[]) {
  return requestClient.post<CodegenFile[]>(
    '/admin/infra/codegen/preview',
    { table_ids: tableIds },
    { timeout: 60_000 },
  );
}

// 下载选中表的生成结果 ZIP（application/zip 流）；大批量同样放宽超时到 60s。
export function downloadCodegen(tableIds: number[]): Promise<Blob> {
  return requestClient.download<Blob>('/admin/infra/codegen/download', {
    data: { table_ids: tableIds },
    method: 'POST',
    timeout: 60_000,
  });
}

// 统一走仓库共享下载 helper（anchor 挂 document + 100ms 延迟 revoke，避免 Safari 取消下载）。
export function saveBlob(blob: Blob, filename: string) {
  downloadFileFromBlob({ fileName: filename, source: blob });
}

// ---- 文件存储（infra file，/admin/infra/file/*；子项目 B / P1-1）----
// 上传走 requestClient——信封拦截器 code===0 解包后即本裸对象。
// public 行带 public_url（/api/public/infra/file/<key>，免鉴权随机键）；
// private 行 public_url 恒空串。
export interface FileMeta {
  id: number;
  name: string;
  size: number;
  mime: string;
  public_url: string;
}

/**
 * 统一上传（multipart：字段 file + 表单域 public/biz_type）。
 * 管理端受控下载用既有 requestClient.download('/admin/infra/file/' + id)；
 * public URL 无需 API 函数，<a href>/<img src> 直连。
 */
export function uploadFile(
  file: File,
  opts?: { bizType?: string; public?: boolean },
) {
  const fd = new FormData();
  fd.append('file', file);
  fd.append('public', String(opts?.public ?? false));
  if (opts?.bizType) {
    fd.append('biz_type', opts.bizType);
  }
  return requestClient.post<FileMeta>('/admin/infra/file/upload', fd);
}
