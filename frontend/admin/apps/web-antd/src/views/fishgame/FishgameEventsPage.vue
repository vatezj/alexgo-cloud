<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Input,
  InputNumber,
  RangePicker,
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import { getFishgameEventsApi } from '#/api/fishgame';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

const filters = ref<{ name: string; playerId: number | undefined; range: any }>({
  name: '',
  playerId: undefined,
  range: undefined,
});

async function load() {
  loading.value = true;
  try {
    const params: Record<string, any> = {
      page: pagination.value.current,
      size: pagination.value.pageSize,
    };
    if (filters.value.name.trim()) params.name = filters.value.name.trim();
    if (filters.value.playerId) params.player_id = filters.value.playerId;
    if (filters.value.range?.length === 2) {
      params.start_t = filters.value.range[0].startOf('second').unix();
      params.end_t = filters.value.range[1].startOf('second').unix();
    }
    const res: any = await getFishgameEventsApi(params);
    list.value = res.list ?? [];
    total.value = res.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function applyFilter() {
  pagination.value.current = 1;
  load();
}

function resetFilter() {
  filters.value = { name: '', playerId: undefined, range: undefined };
  pagination.value.current = 1;
  load();
}

function prettyProps(raw: string) {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}

const nameColor = (name: string) =>
  ({ catch: 'green', empty: 'default', escape: 'orange', broken: 'red' } as Record<string, string>)[name] ?? 'blue';

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex flex-wrap gap-2">
        <Input
          v-model:value="filters.name"
          allow-clear
          class="w-40"
          placeholder="事件名"
          @press-enter="applyFilter"
        />
        <InputNumber
          v-model:value="filters.playerId"
          :min="1"
          class="w-40"
          placeholder="玩家 ID"
          @press-enter="applyFilter"
        />
        <RangePicker v-model:value="filters.range" show-time @press-enter="applyFilter" />
        <Button type="primary" @click="applyFilter">查询</Button>
        <Button @click="resetFilter">重置</Button>
      </div>
      <Table
        :data-source="list"
        :loading="loading"
        :pagination="{
          ...pagination,
          total,
          onChange: (p: number, ps: number) => {
            pagination.current = p;
            pagination.pageSize = ps;
            load();
          },
        }"
        row-key="id"
      >
        <TableColumn title="ID" data-index="id" :width="80" />
        <TableColumn title="玩家" data-index="player_id" :width="90" />
        <TableColumn title="事件" data-index="name" :width="120">
          <template #default="{ record }">
            <Tag :color="nameColor(record.name)">{{ record.name }}</Tag>
          </template>
        </TableColumn>
        <TableColumn title="时间戳 t" data-index="t" :width="120" />
        <TableColumn title="props">
          <template #default="{ record }">
            <details>
              <summary class="cursor-pointer text-xs opacity-70">展开 JSON</summary>
              <pre class="mt-1 max-h-48 overflow-auto rounded bg-gray-100 p-2 text-xs">{{
                prettyProps(record.props)
              }}</pre>
            </details>
          </template>
        </TableColumn>
      </Table>
    </div>
  </Page>
</template>
