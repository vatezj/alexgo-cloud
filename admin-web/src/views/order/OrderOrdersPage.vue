<template>
  <n-space vertical :size="12">
    <n-page-header title="订单列表" subtitle="/api/admin/order/orders" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.user_id" placeholder="user_id" style="max-width: 160px" />
        <n-input v-model:value="createForm.product_id" placeholder="product_id" style="max-width: 160px" />
        <n-input v-model:value="createForm.amount" placeholder="amount" style="max-width: 160px" />
        <n-input v-model:value="createForm.status" placeholder="status" style="max-width: 160px" />
        <n-input v-model:value="createForm.order_no" placeholder="order_no" style="max-width: 180px" />
        <n-button type="primary" :loading="creating" @click="onCreate">创建订单</n-button>
        <n-button :loading="loading" @click="refresh">刷新</n-button>
      </n-space>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :row-key="rowKey" />
    </n-card>
  </n-space>
</template>

<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { onMounted, reactive, ref } from 'vue'
import { createOrder, listOrders, type Order } from '../../api/admin'

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<Order[]>([])

const createForm = reactive({
  user_id: '1',
  product_id: '1',
  amount: '1',
  status: '1',
  order_no: 'demo',
})

const columns: DataTableColumns<Order> = [
  { title: 'ID', key: 'id' },
  { title: 'UserID', key: 'user_id' },
  { title: 'ProductID', key: 'product_id' },
  { title: 'Amount', key: 'amount' },
  { title: 'Status', key: 'status' },
  { title: 'OrderNo', key: 'order_no' },
  { title: 'CreatedAt', key: 'created_at' },
]

function rowKey(row: Order) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listOrders()
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
    await createOrder({
      user_id: Number(createForm.user_id),
      product_id: Number(createForm.product_id),
      amount: Number(createForm.amount),
      status: Number(createForm.status),
      order_no: createForm.order_no,
    })
    message.success('创建成功')
    await refresh()
  } catch (e: any) {
    message.error(`创建失败：${e?.message || 'unknown error'}`)
  } finally {
    creating.value = false
  }
}

onMounted(refresh)
</script>

