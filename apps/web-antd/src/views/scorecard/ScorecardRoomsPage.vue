<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

import { Page } from '@vben/common-ui';

import { Button, message, Modal, Table, TableColumn, Tag } from 'ant-design-vue';

import { closeRoomApi, getRoomsApi } from '#/api/scorecard';

const router = useRouter();
const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

const statusMap: Record<number, { color: string; text: string }> = {
  0: { text: '进行中', color: 'green' },
  1: { text: '已结算', color: 'blue' },
  2: { text: '已关闭', color: 'default' },
};

async function load() {
  loading.value = true;
  try {
    const res = await getRoomsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? res;
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

function openDetail(row: any) {
  router.push({ path: '/scorecard/rooms/detail', query: { id: row.id } });
}

function confirmClose(row: any) {
  Modal.confirm({
    title: '强制关房',
    content: `确认关闭房间 ${row.room_code}？关闭后成员不可再记分。`,
    onOk: async () => {
      await closeRoomApi(row.id);
      message.success('已关闭');
      await load();
    },
  });
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
      >
        <TableColumn title="房号" data-index="room_code" />
        <TableColumn title="状态" data-index="status">
          <template #default="{ record }">
            <Tag :color="statusMap[record.status]?.color">{{ statusMap[record.status]?.text }}</Tag>
          </template>
        </TableColumn>
        <TableColumn title="房主ID" data-index="creator_id" />
        <TableColumn title="茶饭池" data-index="tea_pool_total" />
        <TableColumn title="把数" data-index="round_no" />
        <TableColumn title="创建时间" data-index="created_at" />
        <TableColumn title="操作">
          <template #default="{ record }">
            <Button size="small" type="link" @click="openDetail(record)">详情</Button>
            <Button
              v-if="record.status === 0"
              danger
              size="small"
              type="link"
              @click="confirmClose(record)"
            >
              强制关房
            </Button>
          </template>
        </TableColumn>
      </Table>
    </div>
  </Page>
</template>
