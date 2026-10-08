import { createRouter, createWebHistory } from 'vue-router'
import type { RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '../stores/auth'

import AdminLayout from '../layouts/AdminLayout.vue'
import LoginPage from '../views/LoginPage.vue'
import DashboardPage from '../views/DashboardPage.vue'
import SystemUsersPage from '../views/system/SystemUsersPage.vue'
import SystemRolesPage from '../views/system/SystemRolesPage.vue'
import SystemMenusPage from '../views/system/SystemMenusPage.vue'
import SystemLoginLogsPage from '../views/system/SystemLoginLogsPage.vue'
import SystemOperateLogsPage from '../views/system/SystemOperateLogsPage.vue'
import SystemDeptsPage from '../views/system/SystemDeptsPage.vue'
import SystemPostsPage from '../views/system/SystemPostsPage.vue'
import SystemDictPage from '../views/system/SystemDictPage.vue'
import SystemConfigsPage from '../views/system/SystemConfigsPage.vue'
import SystemNoticesPage from '../views/system/SystemNoticesPage.vue'
import OrderOrdersPage from '../views/order/OrderOrdersPage.vue'

const routes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: LoginPage,
    meta: { public: true },
  },
  {
    path: '/',
    component: AdminLayout,
    children: [
      { path: '', name: 'dashboard', component: DashboardPage },
      { path: 'system/users', name: 'system-users', component: SystemUsersPage },
      { path: 'system/roles', name: 'system-roles', component: SystemRolesPage },
      { path: 'system/menus', name: 'system-menus', component: SystemMenusPage },
      { path: 'system/depts', name: 'system-depts', component: SystemDeptsPage },
      { path: 'system/posts', name: 'system-posts', component: SystemPostsPage },
      { path: 'system/dict', name: 'system-dict', component: SystemDictPage },
      { path: 'system/configs', name: 'system-configs', component: SystemConfigsPage },
      { path: 'system/notices', name: 'system-notices', component: SystemNoticesPage },
      { path: 'system/logins', name: 'system-login-logs', component: SystemLoginLogsPage },
      { path: 'system/operates', name: 'system-operate-logs', component: SystemOperateLogsPage },
      { path: 'order/orders', name: 'order-orders', component: OrderOrdersPage },
    ],
  },
]

export const router = createRouter({
  history: createWebHistory(),
  routes,
})

router.beforeEach((to) => {
  if (to.meta.public) return true
  const auth = useAuthStore()
  if (!auth.token) return { name: 'login', query: { redirect: to.fullPath } }
  return true
})
