import { useQuery } from '@tanstack/react-query'
import { listUsers } from '../api/client'

export function useUsers(token: string) {
  return useQuery({
    queryKey: ['users'],
    queryFn: () => listUsers(token),
    enabled: Boolean(token),
  })
}

