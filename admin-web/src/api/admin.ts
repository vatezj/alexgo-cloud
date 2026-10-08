import { http } from './http'

export type User = {
  id: number
  username?: string
  nickname?: string
  status?: number
  created_at?: string
  updated_at?: string
}

export type Order = {
  id: number
  user_id?: number
  product_id?: number
  amount?: number
  status?: number
  order_no?: string
  created_at?: string
  updated_at?: string
}

export async function login(payload: { username: string; password: string }) {
  return http.post<{ token: string }>('/api/app/system/auth/login', payload)
}

export async function profile() {
  return http.get<{
    user: { user_id: number; username: string; tenant_id: number }
    roles: string[]
    perms: string[]
    menus: any
    routes: any
  }>('/api/admin/system/auth/profile')
}

export async function listUsers() {
  return http.get<{ data: User[] }>('/api/admin/system/users')
}

export async function listOrders() {
  return http.get<{ data: Order[] }>('/api/admin/order/orders')
}

export async function createOrder(payload: {
  user_id: number
  product_id: number
  amount: number
  status: number
  order_no: string
}) {
  return http.post<{ status: string }>('/api/admin/order/orders', payload)
}

export async function createUser(payload: { username: string; nickname: string; password: string }) {
  return http.post<{ data: User }>('/api/admin/system/users', payload)
}

export async function resetUserPassword(userId: number, password: string) {
  return http.post<{ status: string }>(`/api/admin/system/users/${userId}/reset-password`, { password })
}

export async function setUserRoles(userId: number, role_ids: number[]) {
  return http.post<{ status: string }>(`/api/admin/system/users/${userId}/roles`, { role_ids })
}

export async function listLoginLogs(limit = 100) {
  return http.get<{ data: any[] }>(`/api/admin/system/logs/login?limit=${limit}`)
}

export async function listOperateLogs(limit = 100) {
  return http.get<{ data: any[] }>(`/api/admin/system/logs/operate?limit=${limit}`)
}

export async function listDepts() {
  return http.get<{ data: any[] }>('/api/admin/system/depts')
}
export async function createDept(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/depts', payload)
}
export async function deleteDept(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/depts/${id}`)
}

export async function listPosts() {
  return http.get<{ data: any[] }>('/api/admin/system/posts')
}
export async function createPost(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/posts', payload)
}
export async function deletePost(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/posts/${id}`)
}

export async function listDictTypes() {
  return http.get<{ data: any[] }>('/api/admin/system/dict/types')
}
export async function createDictType(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/dict/types', payload)
}
export async function deleteDictType(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/dict/types/${id}`)
}
export async function listDictDatas(typeId: number) {
  return http.get<{ data: any[] }>(`/api/admin/system/dict/datas?type_id=${typeId}`)
}
export async function createDictData(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/dict/datas', payload)
}
export async function deleteDictData(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/dict/datas/${id}`)
}

export async function listConfigs() {
  return http.get<{ data: any[] }>('/api/admin/system/configs')
}
export async function createConfig(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/configs', payload)
}
export async function deleteConfig(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/configs/${id}`)
}

export async function listNotices() {
  return http.get<{ data: any[] }>('/api/admin/system/notices')
}
export async function createNotice(payload: any) {
  return http.post<{ data: any }>('/api/admin/system/notices', payload)
}
export async function deleteNotice(id: number) {
  return http.delete<{ status: string }>(`/api/admin/system/notices/${id}`)
}
