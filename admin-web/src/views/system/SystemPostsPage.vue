<template>
  <n-space vertical :size="12">
    <n-page-header title="岗位管理" subtitle="/api/admin/system/posts" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.code" placeholder="code" style="max-width: 180px" />
        <n-input v-model:value="createForm.name" placeholder="name" style="max-width: 180px" />
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
import { createPost, deletePost, listPosts } from '../../api/admin'

type Post = { id: number; code: string; name: string; sort: number; status: number }

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Post[]>([])
const createForm = reactive({ code: 'dev', name: '开发', sort: '1', status: 1 })

const columns: DataTableColumns<Post> = [
  { title: 'ID', key: 'id' },
  { title: 'Code', key: 'code' },
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

function rowKey(row: Post) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listPosts()
    rows.value = (res.data || []) as Post[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    await createPost({
      code: createForm.code,
      name: createForm.name,
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
    await deletePost(id)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>

