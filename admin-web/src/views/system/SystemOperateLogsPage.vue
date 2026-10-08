<template>
  <n-space vertical :size="12">
    <n-page-header title="操作日志" subtitle="/api/admin/system/logs/operate" />
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
import { listOperateLogs } from '../../api/admin'

type OperateLog = {
  id: number
  tenant_id: number
  user_id: number
  username: string
  method: string
  path: string
  status: number
  latency_ms: number
  error: string
  created_at: string
}

const message = useMessage()
const loading = ref(false)
const rows = ref<OperateLog[]>([])

const columns: DataTableColumns<OperateLog> = [
  { title: 'ID', key: 'id' },
  { title: 'Tenant', key: 'tenant_id' },
  { title: 'User', key: 'username' },
  { title: 'Method', key: 'method' },
  { title: 'Path', key: 'path' },
  { title: 'Status', key: 'status' },
  { title: 'Latency(ms)', key: 'latency_ms' },
  { title: 'Error', key: 'error' },
  { title: 'CreatedAt', key: 'created_at' },
]

function rowKey(row: OperateLog) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listOperateLogs()
    rows.value = (res.data || []) as OperateLog[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

onMounted(refresh)
</script>

