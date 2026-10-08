<template>
  <n-space vertical :size="12">
    <n-page-header title="系统参数" subtitle="/api/admin/system/configs" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.key" placeholder="key" style="max-width: 220px" />
        <n-input v-model:value="createForm.value" placeholder="value" style="max-width: 260px" />
        <n-input v-model:value="createForm.name" placeholder="name" style="max-width: 180px" />
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
import { createConfig, deleteConfig, listConfigs } from '../../api/admin'

type Cfg = { id: number; key: string; value: string; name: string; remark?: string }

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Cfg[]>([])
const createForm = reactive({ key: 'site.name', value: 'alexGo-cloud', name: '站点名称', remark: '' })

const columns: DataTableColumns<Cfg> = [
  { title: 'ID', key: 'id' },
  { title: 'Key', key: 'key' },
  { title: 'Value', key: 'value' },
  { title: 'Name', key: 'name' },
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

function rowKey(row: Cfg) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listConfigs()
    rows.value = (res.data || []) as Cfg[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  creating.value = true
  try {
    await createConfig(createForm)
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
    await deleteConfig(id)
    message.success('删除成功')
    await refresh()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refresh)
</script>

