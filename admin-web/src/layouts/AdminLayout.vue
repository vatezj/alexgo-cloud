<template>
  <n-layout has-sider style="height: 100vh">
    <n-layout-sider bordered collapse-mode="width" :collapsed-width="64" :width="220" show-trigger>
      <div style="height: 56px; display: flex; align-items: center; padding: 0 12px; font-weight: 600">
        alexGo-cloud
      </div>
      <n-menu :options="menuOptions" :value="activeKey" @update:value="onSelect" />
    </n-layout-sider>
    <n-layout>
      <n-layout-header bordered style="height: 56px; display: flex; align-items: center; padding: 0 16px">
        <n-space align="center" justify="space-between" style="width: 100%">
          <div>{{ title }}</div>
          <n-button size="small" @click="logout">退出</n-button>
        </n-space>
      </n-layout-header>
      <n-layout-content style="padding: 16px">
        <router-view />
      </n-layout-content>
      <n-layout-footer bordered style="height: 40px; display: flex; align-items: center; padding: 0 16px">
        <span style="opacity: 0.7">alexGo-cloud Admin</span>
      </n-layout-footer>
    </n-layout>
  </n-layout>
</template>

<script setup lang="ts">
import type { MenuOption } from 'naive-ui'
import { computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { usePermStore } from '../stores/perm'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const perm = usePermStore()

const menuOptions = computed<MenuOption[]>(() => {
  const base: MenuOption[] = [{ label: '仪表盘', key: 'dashboard', path: '/' }]
  const p = perm.perms
  if (p.includes('system:user:list') || p.includes('system')) {
    base.push({ label: '系统用户', key: 'system-users', path: '/system/users' })
  }
  if (p.includes('system:role:*') || p.includes('system')) {
    base.push({ label: '角色管理', key: 'system-roles', path: '/system/roles' })
  }
  if (p.includes('system:menu:*') || p.includes('system')) {
    base.push({ label: '菜单管理', key: 'system-menus', path: '/system/menus' })
  }
  if (p.includes('system:dept:*') || p.includes('system')) {
    base.push({ label: '部门管理', key: 'system-depts', path: '/system/depts' })
  }
  if (p.includes('system:post:*') || p.includes('system')) {
    base.push({ label: '岗位管理', key: 'system-posts', path: '/system/posts' })
  }
  if (p.includes('system:dict:*') || p.includes('system')) {
    base.push({ label: '字典管理', key: 'system-dict', path: '/system/dict' })
  }
  if (p.includes('system:config:*') || p.includes('system')) {
    base.push({ label: '系统参数', key: 'system-configs', path: '/system/configs' })
  }
  if (p.includes('system:notice:*') || p.includes('system')) {
    base.push({ label: '通知公告', key: 'system-notices', path: '/system/notices' })
  }
  if (p.includes('system') || p.length > 0) {
    base.push({ label: '登录日志', key: 'system-login-logs', path: '/system/logins' })
    base.push({ label: '操作日志', key: 'system-operate-logs', path: '/system/operates' })
  }
  base.push({ label: '订单列表', key: 'order-orders', path: '/order/orders' })
  return base
})

const activeKey = computed(() => (route.name as string) || 'dashboard')
const title = computed(() => {
  const hit = menuOptions.value.find((m) => m.key === activeKey.value)
  return hit?.label || '后台管理'
})

function onSelect(key: string, option: MenuOption) {
  const path = (option as any).path as string | undefined
  if (path) router.push(path)
}

function logout() {
  auth.clear()
  perm.clear()
  router.replace({ name: 'login' })
}

onMounted(async () => {
  if (!perm.profile) {
    await perm.load()
  }
})
</script>
