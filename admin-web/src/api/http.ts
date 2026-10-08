import { useAuthStore } from '../stores/auth'

export type HTTPError = {
  status: number
  message: string
  body?: unknown
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const auth = useAuthStore()
  const headers = new Headers(init?.headers || {})
  headers.set('Accept', 'application/json')
  if (!headers.has('Content-Type') && init?.body) headers.set('Content-Type', 'application/json')
  if (auth.token) headers.set('Authorization', `Bearer ${auth.token}`)

  const res = await fetch(path, { ...init, headers })
  const text = await res.text()
  const body = text ? safeJSON(text) : null

  if (!res.ok) {
    const err: HTTPError = { status: res.status, message: res.statusText, body }
    throw err
  }
  return body as T
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

export const http = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, data?: unknown) =>
    request<T>(path, { method: 'POST', body: data === undefined ? undefined : JSON.stringify(data) }),
  delete: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
}
