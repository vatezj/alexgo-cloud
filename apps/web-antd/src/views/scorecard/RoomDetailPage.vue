<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRoute } from 'vue-router';

import { Page } from '@vben/common-ui';

import {
  Descriptions,
  DescriptionsItem,
  Spin,
  Table,
  TableColumn,
} from 'ant-design-vue';

import { getRoomDetailApi } from '#/api/scorecard';

const route = useRoute();
const loading = ref(false);
const detail = ref<any>(null);
const logs = ref<any[]>([]);
const logPagination = ref({ current: 1, pageSize: 20 });
const roomID = Number(route.query.id);

async function load() {
  if (!roomID) return;
  loading.value = true;
  try {
    detail.value = await getRoomDetailApi(roomID);
    logs.value = (detail.value as any)?.logs ?? [];
  } finally {
    loading.value = false;
  }
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4 space-y-4">
      <Spin :spinning="loading">
        <Descriptions v-if="detail" bordered :column="3" size="small">
          <DescriptionsItem label="房号">{{ detail.room?.room_code }}</DescriptionsItem>
          <DescriptionsItem label="状态">{{ ['进行中', '已结算', '已关闭'][detail.room?.status] }}</DescriptionsItem>
          <DescriptionsItem label="房主ID">{{ detail.room?.creator_id }}</DescriptionsItem>
          <DescriptionsItem label="茶饭池">{{ detail.room?.tea_pool_total }}</DescriptionsItem>
          <DescriptionsItem label="把数">{{ detail.room?.round_no }}</DescriptionsItem>
          <DescriptionsItem label="过期时间">{{ detail.room?.expire_time }}</DescriptionsItem>
        </Descriptions>

        <h4 class="mt-4 mb-2 font-semibold">成员</h4>
        <Table :data-source="detail?.members ?? []" :pagination="false" row-key="id" size="small">
          <TableColumn title="用户ID" data-index="user_id" />
          <TableColumn title="当前分" data-index="cur_score" />
          <TableColumn title="累计输分" data-index="cum_lose_score" />
          <TableColumn title="房主">
            <template #default="{ record }">{{ record.is_creator ? '是' : '' }}</template>
          </TableColumn>
        </Table>

        <h4 class="mt-4 mb-2 font-semibold">流水</h4>
        <Table
          :data-source="logs"
          :loading="loading"
          :pagination="{ ...logPagination, total: logs.length, onChange: (p: number, ps: number) => { logPagination.current = p; logPagination.pageSize = ps; } }"
          row-key="id"
          size="small"
        >
          <TableColumn title="类型" data-index="type" />
          <TableColumn title="from" data-index="from_user_id" />
          <TableColumn title="to" data-index="to_user_id" />
          <TableColumn title="分数" data-index="score" />
          <TableColumn title="茶饭" data-index="tea" />
          <TableColumn title="时间" data-index="created_at" />
        </Table>
      </Spin>
    </div>
  </Page>
</template>
