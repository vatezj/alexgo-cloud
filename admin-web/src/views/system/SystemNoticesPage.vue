<template>
  <n-space vertical :size="12">
    <n-page-header title="通知公告" subtitle="/api/admin/system/notices" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.title" placeholder="title" style="max-width: 240px" />
        <n-input v-model:value="createForm.content" placeholder="content" style="max-width: 360px" />
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
import { createNotice, deleteNotice, listNotices } from '../../api/admin'

type Notice = { id: number; title: string; content: string; status: number; created_at?: string }

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Notice[]>([])
const createForm = reactive({ title: '系统公告', content: '欢迎使用 alexGo-cloud', status: 1 })

const columns: DataTableColumns<Notice> = [
  { title: 'ID', key: 'id' },
  { title: 'Title', key: 'title' },
  { title: 'Status', key: 'status' },
  { title: 'CreatedAt', key: 'created_at' },
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

function rowKey(row: Notice) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listNotices()
    rows.value = (res.data || []) as Notice[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    await createNotice(createForm)
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
    await deleteNotice(id)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>

