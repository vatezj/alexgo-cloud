<template>
  <n-space vertical :size="12">
    <n-page-header title="角色管理" subtitle="/api/admin/system/roles" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.code" placeholder="code (e.g. admin)" style="max-width: 200px" />
        <n-input v-model:value="createForm.name" placeholder="name (e.g. 管理员)" style="max-width: 200px" />
        <n-button type="primary" :loading="creating" @click="onCreate">创建</n-button>
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :row-key="rowKey" />
    </n-card>
  </n-space>
</template>

<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { NButton, NSpace, useMessage } from 'naive-ui'
import { h, onMounted, reactive, ref } from 'vue'
import { http } from '../../api/http'

type Role = { id: number; code: string; name: string; status: number }

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Role[]>([])

const createForm = reactive({ code: 'admin', name: '管理员' })

const columns: DataTableColumns<Role> = [
  { title: 'ID', key: 'id' },
  { title: 'Code', key: 'code' },
  { title: 'Name', key: 'name' },
  { title: 'Status', key: 'status' },
  {
    title: 'Actions',
    key: 'actions',
    render(row) {
      return h(
        NSpace,
        {},
        {
          default: () => [
            h(
              NButton,
              { size: 'small', type: 'error', onClick: () => onDelete(row.id) },
              { default: () => '删除' }
            ),
          ],
        }
      )
    },
  },
]

function rowKey(row: Role) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await http.get<{ data: Role[] }>('/api/admin/system/roles')
    rows.value = res.data || []
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    await http.post('/api/admin/system/roles', { code: createForm.code, name: createForm.name })
    message.success('创建成功')
    await refresh()
  } catch (e: any) {
    message.error(`创建失败：${e?.message || 'unknown error'}`)
  } finally {
    creating.value = false
  }
}

async function onDelete(id: number) {
  try {
    await http.delete(`/api/admin/system/roles/${id}`)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>
