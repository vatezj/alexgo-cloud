<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, InputNumber, Table, TableColumn } from 'ant-design-vue';

import { getRecordsApi } from '#/api/scorecard';

const loading = ref(false);
const list = ref<any[]>([]);
const filters = ref<{ room_id?: number; user_id?: number }>({});
const pagination = ref({ current: 1, pageSize: 20 });
const total = ref(0);

const settleTypeMap: Record<number, string> = {
  1: '正常结算',
  2: '房主退房自动',
  3: '退房',
  4: '空战绩',
};

async function load() {
  loading.value = true;
  try {
    const res = await getRecordsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
      ...(filters.value.user_id ? { user_id: filters.value.user_id } : {}),
      ...(filters.value.room_id ? { room_id: filters.value.room_id } : {}),
    });
    list.value = (res as any).list ?? [];
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

function filterByRoom(row: any) {
  filters.value = { room_id: row.room_id };
  pagination.value.current = 1;
  load();
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <InputNumber v-model:value="filters.user_id" placeholder="用户ID" :min="1" class="w-40" />
        <InputNumber v-model:value="filters.room_id" placeholder="房间ID" :min="1" class="w-40" />
        <Button type="primary" @click="pagination.current = 1; load()">查询</Button>
        <Button @click="filters = {}; pagination.current = 1; load()">重置</Button>
      </div>
      <Table
        :data-source="list"
        :loading="loading"
        :pagination="{ ...pagination, total, onChange: (p: number, ps: number) => { pagination.current = p; pagination.pageSize = ps; load(); } }"
        row-key="id"
        size="small"
      >
        <TableColumn title="战绩ID" data-index="id" />
        <TableColumn title="房间ID" data-index="room_id" />
        <TableColumn title="用户ID" data-index="user_id" />
        <TableColumn title="终局分" data-index="score" />
        <TableColumn title="累计输分" data-index="cum_lose_score" />
        <TableColumn title="茶饭池" data-index="tea_pool_total" />
        <TableColumn title="结算类型">
          <template #default="{ record }">{{ settleTypeMap[record.settle_type] ?? record.settle_type }}</template>
        </TableColumn>
        <TableColumn title="时间" data-index="created_at" />
        <TableColumn title="操作">
          <template #default="{ record }">
            <Button size="small" type="link" @click="filterByRoom(record)">看全房</Button>
          </template>
        </TableColumn>
      </Table>
    </div>
  </Page>
</template>
