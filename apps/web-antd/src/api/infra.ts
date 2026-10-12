import { downloadFileFromBlob } from '@vben/utils';

import { requestClient } from './request';

// ---- 代码生成（infra codegen，/admin/infra/codegen/*）----
// 信封 {data,total} 被 requestClient 拦截器解包后 total 丢失：
// 对齐 admin.ts listClients，一次拉满上限 200，页面用 localPage 本地切页。

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

export function listCodegenTables() {
  return requestClient.get<CodegenTable[]>(
    '/admin/infra/codegen/tables?page=1&size=200',
  );
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

// 响应 {data,columns} 解包后只剩 table（columns 丢失）——列配置走
// listCodegenColumns 单独拉，二者配合覆盖详情所需。
export function getCodegenTable(id: number) {
  return requestClient.get<CodegenTable>(`/admin/infra/codegen/tables/${id}`);
}

export function updateCodegenTable(id: number, payload: Partial<CodegenTable>) {
  return requestClient.put<{ status: string }>(
    `/admin/infra/codegen/tables/${id}`,
    payload,
  );
}

export function deleteCodegenTable(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/infra/codegen/tables/${id}`,
  );
}

export function listCodegenColumns(id: number) {
  return requestClient.get<CodegenColumn[]>(
    `/admin/infra/codegen/tables/${id}/columns`,
  );
}

// 全量保存：后端按提交数组整体替换该表配置。
export function saveCodegenColumns(id: number, columns: CodegenColumn[]) {
  return requestClient.put<{ status: string }>(
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
export function previewCodegen(tableIds: number[]) {
  return requestClient.post<CodegenFile[]>('/admin/infra/codegen/preview', {
    table_ids: tableIds,
  });
}

// 下载选中表的生成结果 ZIP（application/zip 流）。
export function downloadCodegen(tableIds: number[]): Promise<Blob> {
  return requestClient.download<Blob>('/admin/infra/codegen/download', {
    data: { table_ids: tableIds },
    method: 'POST',
  });
}

// 统一走仓库共享下载 helper（anchor 挂 document + 100ms 延迟 revoke，避免 Safari 取消下载）。
export function saveBlob(blob: Blob, filename: string) {
  downloadFileFromBlob({ fileName: filename, source: blob });
}
