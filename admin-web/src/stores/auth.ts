import { defineStore } from 'pinia'

const TOKEN_KEY = 'alexgo_admin_token'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    token: localStorage.getItem(TOKEN_KEY) || '',
  }),
  actions: {
    setToken(token: string) {
      this.token = token
      localStorage.setItem(TOKEN_KEY, token)
    },
    clear() {
      this.token = ''
      localStorage.removeItem(TOKEN_KEY)
    },
  },
})

