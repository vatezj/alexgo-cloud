/**
 * alexGo-cloud 管理端接口：承接旧 admin-web 的全部调用（路径去掉 /api 前缀——
 * baseURL = VITE_GLOB_API_URL = /api）。统一信封 {code,msg,data} 由
 * src/api/request.ts 拦截器解包——到这里都是裸数据（data 字段）。
 */
import { saveBlob } from '#/api/infra';
import { requestClient } from '#/api/request';

export interface User {
  id: number;
  username: string;
  nickname: string;
  status: number;
  created_at: string;
  updated_at: string;
}

export interface Role {
  id: number;
  code: string;
  name: string;
  status: number;
}

export interface Menu {
  id: number;
  parent_id: number;
  type: string; // dir | menu | button
  name: string;
  path: string;
  component: string;
  icon: string;
  permission: string;
  sort: number;
  status: number;
}

export interface Dept {
  id: number;
  parent_id: number;
  name: string;
  sort: number;
  status: number;
}

export interface Post {
  id: number;
  code: string;
  name: string;
  sort: number;
  status: number;
}

export interface DictType {
  id: number;
  type: string;
  name: string;
  status: number;
}

export interface DictData {
  id: number;
  type_id: number;
  label: string;
  value: string;
  sort: number;
  status: number;
}

export interface SystemConfig {
  id: number;
  key: string;
  value: string;
  name: string;
  remark?: string;
}

export interface Notice {
  attachments: string; // 附件 JSON 数组文本（可为 ''——老数据无附件）
  content: string;
  created_at: string;
  id: number;
  status: number;
  title: string;
}

export interface NoticeItem extends Notice {
  read_status: number;
}

export interface LoginLog {
  id: number;
  tenant_id: number;
  username: string;
  user_id: number;
  ip: string;
  success: number;
  message: string;
  created_at: string;
}

export interface OperateLog {
  id: number;
  tenant_id: number;
  username: string;
  method: string;
  path: string;
  status: number;
  latency_ms: number;
  error: string;
  created_at: string;
}

export interface Order {
  id: number;
  user_id: number;
  product_id: number;
  amount: number;
  status: number;
  order_no: string;
  created_at: string;
}

// ---- 用户 ----
export function listUsers() {
  return requestClient.get<User[]>('/admin/system/users');
}

export function createUser(payload: {
  nickname: string;
  password: string;
  username: string;
}) {
  return requestClient.post<User>('/admin/system/users', payload);
}

export function resetUserPassword(
  userId: number,
  password: string,
): Promise<void> {
  return requestClient.post(`/admin/system/users/${userId}/reset-password`, {
    password,
  });
}

// ---- 角色 ----
export function listRoles() {
  return requestClient.get<Role[]>('/admin/system/roles');
}

export function createRole(payload: { code: string; name: string }) {
  return requestClient.post<Role>('/admin/system/roles', payload);
}

export function deleteRole(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/roles/${id}`);
}

// ---- 菜单 ----
/** 新建/编辑菜单的表单载荷（编辑时 id 走 URL 路径参数） */
export interface MenuPayload {
  component: string;
  icon: string;
  name: string;
  parent_id: number;
  path: string;
  permission: string;
  sort: number;
  status: number;
  type: string;
}

export function listMenus() {
  return requestClient.get<Menu[]>('/admin/system/menus');
}

export function createMenu(payload: MenuPayload) {
  return requestClient.post<Menu>('/admin/system/menus', payload);
}

export function updateMenu(id: number, payload: MenuPayload) {
  return requestClient.put<Menu>(`/admin/system/menus/${id}`, payload);
}

export function deleteMenu(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/menus/${id}`);
}

// ---- 部门 ----
export function listDepts() {
  return requestClient.get<Dept[]>('/admin/system/depts');
}

export function createDept(payload: {
  name: string;
  parent_id: number;
  sort: number;
  status: number;
}) {
  return requestClient.post<Dept>('/admin/system/depts', payload);
}

export function deleteDept(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/depts/${id}`);
}

// ---- 岗位 ----
export function listPosts() {
  return requestClient.get<Post[]>('/admin/system/posts');
}

export function createPost(payload: {
  code: string;
  name: string;
  sort: number;
  status: number;
}) {
  return requestClient.post<Post>('/admin/system/posts', payload);
}

export function deletePost(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/posts/${id}`);
}

// ---- 字典 ----
export function listDictTypes() {
  return requestClient.get<DictType[]>('/admin/system/dict/types');
}

export function createDictType(payload: {
  name: string;
  status: number;
  type: string;
}) {
  return requestClient.post<DictType>('/admin/system/dict/types', payload);
}

export function deleteDictType(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/dict/types/${id}`);
}

export function listDictDatas(typeId: number) {
  return requestClient.get<DictData[]>(
    `/admin/system/dict/datas?type_id=${typeId}`,
  );
}

export function createDictData(payload: {
  label: string;
  sort: number;
  status: number;
  type_id: number;
  value: string;
}) {
  return requestClient.post<DictData>('/admin/system/dict/datas', payload);
}

export function deleteDictData(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/dict/datas/${id}`);
}

// ---- 系统参数 ----
export function listConfigs() {
  return requestClient.get<SystemConfig[]>('/admin/system/configs');
}

export function createConfig(payload: {
  key: string;
  name: string;
  remark: string;
  value: string;
}) {
  return requestClient.post<SystemConfig>('/admin/system/configs', payload);
}

export function deleteConfig(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/configs/${id}`);
}

// ---- 通知公告 ----
export function listNotices() {
  return requestClient.get<Notice[]>('/admin/system/notices');
}

export function createNotice(payload: {
  attachments: string;
  content: string;
  status: number;
  title: string;
}) {
  return requestClient.post<Notice>('/admin/system/notices', payload);
}

export function deleteNotice(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/notices/${id}`);
}

// ---- 我的通知（已读回执） ----
export function listMyNotices() {
  return requestClient.get<NoticeItem[]>('/admin/system/notices/mine');
}

export function markNoticeRead(id: number): Promise<void> {
  return requestClient.post(`/admin/system/notices/${id}/read`);
}

export function markAllNoticesRead(): Promise<void> {
  return requestClient.post('/admin/system/notices/read-all');
}

// ---- 日志 ----
export function listLoginLogs(limit = 100) {
  return requestClient.get<LoginLog[]>(
    `/admin/system/logs/login?limit=${limit}`,
  );
}

export function listOperateLogs(limit = 100) {
  return requestClient.get<OperateLog[]>(
    `/admin/system/logs/operate?limit=${limit}`,
  );
}

// ---- 订单 ----
export function listOrders() {
  return requestClient.get<Order[]>('/admin/order/orders');
}

export function createOrder(payload: {
  amount: number;
  order_no: string;
  product_id: number;
  status: number;
  user_id: number;
}) {
  return requestClient.post<Order>('/admin/order/orders', payload);
}

// ---- OAuth2 客户端 ----
export interface Oauth2Client {
  id: number;
  client_id: string;
  client_name: string;
  website: string;
  grant_types: string; // 逗号分隔
  redirect_uris: string; // 换行分隔，精确匹配
  account_types: string; // 逗号分隔 system,member
  scopes: string;
  activity: number; // 1 启用 0 停用
  created_at: string;
  updated_at: string;
}

export interface Oauth2ClientPayload {
  client_id: string;
  client_secret?: string; // 明文；编辑留空=不改（后端 Update 语义）
  client_name: string;
  website: string;
  grant_types: string;
  redirect_uris: string;
  account_types: string;
  scopes: string;
  activity: number;
}

// 服务端真分页（resp.Page 信封 data:{list,total,page,size}，size 上限 100）。
export function listOauth2ClientsPage(params: { page: number; size: number }) {
  return requestClient.get<{ list: Oauth2Client[]; total: number }>(
    '/admin/oauth2/clients',
    { params },
  );
}

// 明文 secret 仅创建响应回显一次（createClientResponse），List/Update 恒不回。
export function createClient(payload: Oauth2ClientPayload) {
  return requestClient.post<Oauth2Client & { client_secret?: string }>(
    '/admin/oauth2/clients',
    payload,
  );
}

export function updateClient(id: number, payload: Oauth2ClientPayload) {
  return requestClient.put<Oauth2Client>(
    `/admin/oauth2/clients/${id}`,
    payload,
  );
}

export function deleteClient(id: number): Promise<void> {
  return requestClient.delete(`/admin/oauth2/clients/${id}`);
}

// ---- 租户 ----
export interface Tenant {
  id: number;
  name: string;
  domain: string;
  status: number; // 1 启用 0 停用
  account_limit: number; // -1 不限（Create 时 0 也会归一为 -1）
  package_id: number; // 展示字段，Update 载荷不含它
  expire_time: null | string; // 展示字段，Update 载荷不含它
  created_at: string;
  updated_at: string;
}

// Create 忽略 status（恒 1 启用）；Update 四字段全量生效。
export interface TenantPayload {
  name: string;
  domain: string;
  status: number;
  account_limit: number;
}

// controller 一次性返回全量（无分页参数）：页面 localPage 本地切页。
export function listTenants() {
  return requestClient.get<Tenant[]>('/admin/system/tenants');
}

export function createTenant(payload: TenantPayload) {
  return requestClient.post<Tenant>('/admin/system/tenants', payload);
}

export function updateTenant(
  id: number,
  payload: TenantPayload,
): Promise<void> {
  return requestClient.put(`/admin/system/tenants/${id}`, payload);
}

export function deleteTenant(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/tenants/${id}`);
}

// ---- Excel 导出（P1-2）----
// download 默认 GET + responseType blob（@vben FileDownloader）；60s 放宽同 codegen。
export function exportUsers(): Promise<Blob> {
  return requestClient.download<Blob>('/admin/system/users/export', {
    timeout: 60_000,
  });
}

export function exportOrders(): Promise<Blob> {
  return requestClient.download<Blob>('/admin/order/orders/export', {
    timeout: 60_000,
  });
}

// saveExportBlob 统一落盘文件名 <base>_<yyyyMMdd_HHmmss>.xlsx（与服务端
// Content-Disposition 同构；浏览器实际保存名以客户端为准）。
export function saveExportBlob(blob: Blob, base: string) {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  const stamp = `${d.getFullYear()}${p(d.getMonth() + 1)}${p(d.getDate())}_${p(d.getHours())}${p(d.getMinutes())}${p(d.getSeconds())}`;
  saveBlob(blob, `${base}_${stamp}.xlsx`);
}

// ---- 在线用户（P1-3） ----
export interface OnlineSession {
  id: number;
  user_id: number;
  username: string;
  client_id: string;
  is_impersonated: boolean;
  issued_at: string;
  expires_at: string;
}

export function listOnlineApi(params: Record<string, any>) {
  return requestClient.get<{ list: OnlineSession[]; total: number }>(
    '/admin/system/online',
    { params },
  );
}

export function kickOnlineApi(id: number): Promise<void> {
  return requestClient.delete(`/admin/system/online/${id}`);
}

export function impersonateApi(userId: number) {
  return requestClient.post<{
    access_token: string;
    expires_in: number;
    refresh_token: string;
  }>('/admin/system/online/impersonate', { user_id: userId });
}

export interface MemberBrief {
  id: number;
  nickname: string;
  mobile: string;
  status: number;
}

export function listMemberUsersApi(params: Record<string, any>) {
  return requestClient.get<{ list: MemberBrief[]; total: number }>(
    '/admin/member/users',
    { params },
  );
}

// ---- 社交登录渠道配置（微信一期） ----
export interface SocialApp {
  id: number;
  tenant_id: number;
  platform: string; // wechat_mp | wechat_web
  appid: string;
  status: number; // 1 启用 0 停用
  create_time: string;
  update_time: string;
  // secret 恒不回显（后端 json:"-"）——故接口不声明该字段
}

export interface SocialAppPayload {
  platform: string;
  appid: string;
  secret?: string; // 明文；编辑留空=保留旧值（后端 Update 语义）
  status: number;
}

// 服务端真分页（resp.Page 信封 data:{list,total,page,size}）；platform=='' 全平台视图。
export function listSocialAppsPage(params: {
  page: number;
  platform?: string;
  size: number;
}) {
  return requestClient.get<{ list: SocialApp[]; total: number }>(
    '/admin/social/apps',
    { params },
  );
}

export function createSocialApp(payload: SocialAppPayload) {
  return requestClient.post<SocialApp>('/admin/social/apps', payload);
}

export function updateSocialApp(id: number, payload: SocialAppPayload) {
  return requestClient.put<SocialApp>(`/admin/social/apps/${id}`, payload);
}

export function deleteSocialApp(id: number): Promise<void> {
  return requestClient.delete(`/admin/social/apps/${id}`);
}
