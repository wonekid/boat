import { defineStore } from 'pinia'

export const useUserStore = defineStore('user', {
  state: () => ({
    token: localStorage.getItem('boat_token') || '',
    user: {} as any,
    roles: [] as any[],
    permissions: [] as string[],
  }),
  getters: {
    isLogin: (s) => !!s.token,
  },
  actions: {
    setToken(t: string) {
      this.token = t
      localStorage.setItem('boat_token', t)
    },
    setUser(u: any) {
      this.user = u
      this.roles = u.roles || []
      // 收集权限标识
      const perms: string[] = []
      ;(this.roles as any[]).forEach((r: any) => {
        ;(r.menus || []).forEach((m: any) => {
          if (m.permission) perms.push(m.permission)
        })
      })
      this.permissions = Array.from(new Set(perms))
    },
    hasPerm(perm: string) {
      return this.permissions.includes(perm)
    },
    logout() {
      this.token = ''
      this.user = {}
      this.roles = []
      this.permissions = []
      localStorage.removeItem('boat_token')
    },
  },
})
