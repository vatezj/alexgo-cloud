/**
 * alexGo-cloud 管理端接口：承接旧 admin-web 的全部调用（路径去掉 /api 前缀——
 * baseURL = VITE_GLOB_API_URL = /api）。响应三形态由 src/api/request.ts 的
 * 拦截器解包：{"code":0,...}/{"data":...}/{"status":"ok"} 到这里都是裸数据。
 */
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
  id: number;
  title: string;
  content: string;
  status: number;
  created_at: string;
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

export function resetUserPassword(userId: number, password: string) {
  return requestClient.post<{ status: string }>(
    `/admin/system/users/${userId}/reset-password`,
    { password },
  );
}

// ---- 角色 ----
export function listRoles() {
  return requestClient.get<Role[]>('/admin/system/roles');
}

export function createRole(payload: { code: string; name: string }) {
  return requestClient.post<Role>('/admin/system/roles', payload);
}

export function deleteRole(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/roles/${id}`);
}

// ---- 菜单 ----
export function listMenus() {
  return requestClient.get<Menu[]>('/admin/system/menus');
}

export function createMenu(payload: {
  component: string;
  icon: string;
  name: string;
  parent_id: number;
  path: string;
  permission: string;
  sort: number;
  status: number;
  type: string;
}) {
  return requestClient.post<Menu>('/admin/system/menus', payload);
}

export function deleteMenu(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/menus/${id}`);
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

export function deleteDept(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/depts/${id}`);
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

export function deletePost(id: number) {
  return requestClient.delete<{ status: string }>(`/admin/system/posts/${id}`);
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

export function deleteDictType(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/dict/types/${id}`,
  );
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

export function deleteDictData(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/dict/datas/${id}`,
  );
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

export function deleteConfig(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/configs/${id}`,
  );
}

// ---- 通知公告 ----
export function listNotices() {
  return requestClient.get<Notice[]>('/admin/system/notices');
}

export function createNotice(payload: {
  content: string;
  status: number;
  title: string;
}) {
  return requestClient.post<Notice>('/admin/system/notices', payload);
}

export function deleteNotice(id: number) {
  return requestClient.delete<{ status: string }>(
    `/admin/system/notices/${id}`,
  );
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
