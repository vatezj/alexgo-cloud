import { defineStore } from 'pinia'
import { profile } from '../api/admin'

export type Profile = {
  user: { user_id: number; username: string; tenant_id: number }
  roles: string[]
  perms: string[]
  menus: any
}

export const usePermStore = defineStore('perm', {
  state: () => ({
    profile: null as Profile | null,
  }),
  getters: {
    perms(state) {
      return state.profile?.perms || []
    },
    hasPerm: (state) => {
      return (code: string) => (state.profile?.perms || []).includes(code)
    },
  },
  actions: {
    async load() {
      this.profile = await profile()
    },
    clear() {
      this.profile = null
    },
  },
})

