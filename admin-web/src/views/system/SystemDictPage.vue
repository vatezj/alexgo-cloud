<template>
  <n-space vertical :size="12">
    <n-page-header title="字典管理" subtitle="/api/admin/system/dict/*" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="typeForm.type" placeholder="type (e.g. user_status)" style="max-width: 200px" />
        <n-input v-model:value="typeForm.name" placeholder="name" style="max-width: 200px" />
        <n-button type="primary" :loading="creatingType" @click="onCreateType">创建字典类型</n-button>
        <n-button :loading="loadingTypes" @click="refreshTypes">刷新类型</n-button>
      </n-space>
      <n-data-table :columns="typeColumns" :data="types" :loading="loadingTypes" :row-key="rowKey" />
    </n-card>

    <n-card title="字典数据">
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="dataForm.type_id" placeholder="type_id" style="max-width: 120px" />
        <n-input v-model:value="dataForm.label" placeholder="label" style="max-width: 160px" />
        <n-input v-model:value="dataForm.value" placeholder="value" style="max-width: 160px" />
        <n-input v-model:value="dataForm.sort" placeholder="sort" style="max-width: 120px" />
        <n-button type="primary" :loading="creatingData" @click="onCreateData">创建字典数据</n-button>
        <n-button :loading="loadingDatas" @click="refreshDatas">刷新数据</n-button>
      </n-space>
      <n-data-table :columns="dataColumns" :data="datas" :loading="loadingDatas" :row-key="rowKey" />
    </n-card>
  </n-space>
</template>

<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { NButton, NSpace, useMessage } from 'naive-ui'
import { h, onMounted, reactive, ref } from 'vue'
import {
  createDictData,
  createDictType,
  deleteDictData,
  deleteDictType,
  listDictDatas,
  listDictTypes,
} from '../../api/admin'

type DictType = { id: number; type: string; name: string; status: number }
type DictData = { id: number; type_id: number; label: string; value: string; sort: number; status: number }

const message = useMessage()

const loadingTypes = ref(false)
const creatingType = ref(false)
const types = ref<DictType[]>([])
const typeForm = reactive({ type: 'user_status', name: '用户状态', status: 1 })

const loadingDatas = ref(false)
const creatingData = ref(false)
const datas = ref<DictData[]>([])
const dataForm = reactive({ type_id: '0', label: '正常', value: '1', sort: '1', status: 1 })

const typeColumns: DataTableColumns<DictType> = [
  { title: 'ID', key: 'id' },
  { title: 'Type', key: 'type' },
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
              { size: 'small', type: 'error', onClick: () => onDeleteType(row.id) },
              { default: () => '删除' }
            ),
            h(
              NButton,
              {
                size: 'small',
                onClick: async () => {
                  dataForm.type_id = String(row.id)
                  await refreshDatas()
                },
              },
              { default: () => '查看数据' }
            ),
          ],
        }
      )
    },
  },
]

const dataColumns: DataTableColumns<DictData> = [
  { title: 'ID', key: 'id' },
  { title: 'TypeID', key: 'type_id' },
  { title: 'Label', key: 'label' },
  { title: 'Value', key: 'value' },
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
              { size: 'small', type: 'error', onClick: () => onDeleteData(row.id) },
              { default: () => '删除' }
            ),
          ],
        }
      )
    },
  },
]

function rowKey(row: any) {
  return row.id
}

async function refreshTypes() {
  loadingTypes.value = true
  try {
    const res = await listDictTypes()
    types.value = (res.data || []) as DictType[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loadingTypes.value = false
  }
}

async function onCreateType() {
  creatingType.value = true
  try {
    await createDictType(typeForm)
    message.success('创建成功')
    await refreshTypes()
  } catch (e: any) {
    message.error(`创建失败：${e?.message || 'unknown error'}`)
  } finally {
    creatingType.value = false
  }
}

async function onDeleteType(id: number) {
  try {
    await deleteDictType(id)
    message.success('删除成功')
    await refreshTypes()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

async function refreshDatas() {
  const tid = Number(dataForm.type_id)
  if (!tid) {
    message.error('请先选择 type_id')
    return
  }
  loadingDatas.value = true
  try {
    const res = await listDictDatas(tid)
    datas.value = (res.data || []) as DictData[]
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loadingDatas.value = false
  }
}

async function onCreateData() {
  const tid = Number(dataForm.type_id)
  if (!tid) {
    message.error('请先选择 type_id')
    return
  }
  creatingData.value = true
  try {
    await createDictData({
      type_id: tid,
      label: dataForm.label,
      value: dataForm.value,
      sort: Number(dataForm.sort),
      status: dataForm.status,
    })
    message.success('创建成功')
    await refreshDatas()
  } catch (e: any) {
    message.error(`创建失败：${e?.message || 'unknown error'}`)
  } finally {
    creatingData.value = false
  }
}

async function onDeleteData(id: number) {
  try {
    await deleteDictData(id)
    message.success('删除成功')
    await refreshDatas()
  } catch (e: any) {
    message.error(`删除失败：${e?.message || 'unknown error'}`)
  }
}

onMounted(refreshTypes)
</script>

