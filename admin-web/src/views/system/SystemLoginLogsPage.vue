<template>
  <n-space vertical :size="12">
    <n-page-header title="登录日志" subtitle="/api/admin/system/logs/login" />
    <n-card>
      <n-space justify="end" style="margin-bottom: 12px">
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :row-key="rowKey" />
    </n-card>
  </n-space>
</template>

<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onMounted, ref } from 'vue'
import { listLoginLogs } from '../../api/admin'

type LoginLog = {
  id: number
  tenant_id: number
  username: string
  user_id: number
  ip: string
  user_agent: string
  success: number
  message: string
  created_at: string
}

const message = useMessage()
const loading = ref(false)
const rows = ref<LoginLog[]>([])

const columns: DataTableColumns<LoginLog> = [
  { title: 'ID', key: 'id' },
  { title: 'Tenant', key: 'tenant_id' },
  { title: 'Username', key: 'username' },
  { title: 'UserID', key: 'user_id' },
  { title: 'IP', key: 'ip' },
  { title: 'Success', key: 'success' },
  { title: 'Message', key: 'message' },
  { title: 'CreatedAt', key: 'created_at' },
]

function rowKey(row: LoginLog) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listLoginLogs()
    rows.value = (res.data || []) as LoginLog[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

