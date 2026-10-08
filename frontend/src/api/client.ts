export type User = { id: number; username?: string }
export type Order = { id: number; order_no?: string }

export async function listUsers(token: string) {
  const res = await fetch('/api/admin/system/users', {
    headers: { Authorization: `Bearer ${token}` },
  })
  if (!res.ok) throw new Error('listUsers failed')
  return res.json()
}

