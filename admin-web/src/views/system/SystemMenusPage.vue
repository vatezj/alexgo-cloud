<template>
  <n-space vertical :size="12">
    <n-page-header title="菜单管理" subtitle="/api/admin/system/menus" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.name" placeholder="name" style="max-width: 180px" />
        <n-input v-model:value="createForm.path" placeholder="path (e.g. /system/users)" style="max-width: 260px" />
        <n-input v-model:value="createForm.permission" placeholder="permission (e.g. system:user:list)" style="max-width: 260px" />
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

type Menu = {
  id: number
  parent_id: number
  type: string
  name: string
  path: string
  permission: string
  sort: number
  status: number
}

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Menu[]>([])

const createForm = reactive({
  parent_id: 0,
  type: 'menu',
  name: '用户管理',
  path: '/system/users',
  component: '',
  icon: '',
  permission: 'system:user:list',
  sort: 1,
  status: 1,
})

const columns: DataTableColumns<Menu> = [
  { title: 'ID', key: 'id' },
  { title: 'ParentID', key: 'parent_id' },
  { title: 'Type', key: 'type' },
  { title: 'Name', key: 'name' },
  { title: 'Path', key: 'path' },
  { title: 'Perm', key: 'permission' },
  { title: 'Sort', key: 'sort' },
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

function rowKey(row: Menu) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await http.get<{ data: Menu[] }>('/api/admin/system/menus')
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
    await http.post('/api/admin/system/menus', createForm)
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
    await http.delete(`/api/admin/system/menus/${id}`)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>

