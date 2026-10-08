<template>
  <n-space vertical :size="12">
    <n-page-header title="部门管理" subtitle="/api/admin/system/depts" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.name" placeholder="name" style="max-width: 200px" />
        <n-input v-model:value="createForm.parent_id" placeholder="parent_id" style="max-width: 140px" />
        <n-input v-model:value="createForm.sort" placeholder="sort" style="max-width: 120px" />
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
import { createDept, deleteDept, listDepts } from '../../api/admin'

type Dept = { id: number; parent_id: number; name: string; sort: number; status: number }

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Dept[]>([])
const createForm = reactive({ name: '研发部', parent_id: '0', sort: '1', status: 1 })

const columns: DataTableColumns<Dept> = [
  { title: 'ID', key: 'id' },
  { title: 'ParentID', key: 'parent_id' },
  { title: 'Name', key: 'name' },
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

function rowKey(row: Dept) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listDepts()
    rows.value = (res.data || []) as Dept[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    await createDept({
      name: createForm.name,
      parent_id: Number(createForm.parent_id),
      sort: Number(createForm.sort),
      status: createForm.status,
    })
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
    await deleteDept(id)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>

