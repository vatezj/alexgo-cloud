<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Table, TableColumn } from 'ant-design-vue';

import { getFriendsApi } from '#/api/scorecard';

const loading = ref(false);
const list = ref<any[]>([]);
const pagination = ref({ current: 1, pageSize: 20 });
const total = ref(0);

async function load() {
  loading.value = true;
  try {
    const res = await getFriendsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? [];
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <Table
        :data-source="list"
        :loading="loading"
        :pagination="{ ...pagination, total, onChange: (p: number, ps: number) => { pagination.current = p; pagination.pageSize = ps; load(); } }"
        row-key="id"
        size="small"
      >
        <TableColumn title="ID" data-index="id" />
        <TableColumn title="用户ID" data-index="user_id" />
        <TableColumn title="好友ID" data-index="friend_user_id" />
        <TableColumn title="参与排名">
          <template #default="{ record }">{{ record.rank_enabled ? '是' : '否' }}</template>
        </TableColumn>
        <TableColumn title="创建时间" data-index="created_at" />
      </Table>
    </div>
  </Page>
</template>
