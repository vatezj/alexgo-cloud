<script lang="ts" setup>
import type { TaokoulingLog } from '#/api/taokouling';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { reactive } from 'vue';

import { Page } from '@vben/common-ui';

import { Button, DatePicker, Input, Select } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listLogsApi } from '#/api/taokouling';
import { formatDateTime } from '#/utils/format';

defineOptions({ name: 'TaokoulingLogsPage' });

/** status：-1 全部（不传，后端缺省 -1 不过滤）/ 1 成功 / 2 降级 / 3 无口令 / 4 硬错误 */
const STATUS_OPTIONS = [
  { label: '全部', value: -1 },
  { label: '成功', value: 1 },
  { label: '降级', value: 2 },
  { label: '无口令', value: 3 },
  { label: '硬错误', value: 4 },
];

const ENDPOINT_LABELS: Record<string, string> = {
  convert: '转换',
  parse: '解析',
};

const STATUS_LABELS: Record<number, string> = {
  1: '成功',
  2: '降级',
  3: '无口令',
  4: '硬错误',
};

// 日期筛选：date-only YYYY-MM-DD（后端 end 的 date-only 含当日全天）；空 = 不过滤
const filters = reactive({
  end: '',
  item_id: '',
  partner: '',
  start: '',
  status: -1,
});

const gridOptions: VxeGridProps<TaokoulingLog> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '时间',
      width: 170,
    },
    {
      field: 'endpoint',
      formatter: ({ cellValue }) =>
        ENDPOINT_LABELS[String(cellValue)] ?? String(cellValue ?? ''),
      title: '端点',
      width: 90,
    },
    { field: 'partner', title: 'partner', width: 100 },
    { field: 'item_id', title: '商品ID', minWidth: 120 },
    {
      field: 'status',
      formatter: ({ cellValue }) =>
        STATUS_LABELS[Number(cellValue)] ?? String(cellValue ?? ''),
      title: '状态',
      width: 90,
    },
    { field: 'duration_ms', title: '耗时(ms)', width: 100 },
    { field: 'ip', title: 'IP', width: 130 },
    // 长文本：全局 showOverflow 已开（省略号 + hover tooltip），列上再显式声明
    {
      field: 'input_text',
      minWidth: 220,
      showOverflow: true,
      title: '输入文本',
    },
    { field: 'error', minWidth: 180, showOverflow: true, title: '错误' },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        // 服务端真分页 + 筛选：空值不入参（status 全部 = 不传，后端缺省 -1）
        const params: Record<string, any> = {
          page: page.currentPage,
          size: page.pageSize,
        };
        if (filters.start) params.start = filters.start;
        if (filters.end) params.end = filters.end;
        const partner = filters.partner.trim();
        if (partner) params.partner = partner;
        const itemId = filters.item_id.trim();
        if (itemId) params.item_id = itemId;
        if (filters.status !== -1) params.status = filters.status;
        const { list, total } = await listLogsApi(params);
        return { items: list, total };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

function onSearch() {
  // reload 重置到第 1 页再查（query 保留当前页）
  void gridApi.reload();
}

function onReset() {
  filters.end = '';
  filters.item_id = '';
  filters.partner = '';
  filters.start = '';
  filters.status = -1;
  void gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <DatePicker
          v-model:value="filters.start"
          class="mr-2"
          placeholder="开始日期"
          style="width: 140px"
          value-format="YYYY-MM-DD"
        />
        <DatePicker
          v-model:value="filters.end"
          class="mr-2"
          placeholder="结束日期（含当日）"
          style="width: 160px"
          value-format="YYYY-MM-DD"
        />
        <Input
          v-model:value="filters.partner"
          allow-clear
          class="mr-2 w-32"
          placeholder="partner"
          @press-enter="onSearch"
        />
        <Select
          v-model:value="filters.status"
          class="mr-2 w-32"
          :options="STATUS_OPTIONS"
          @change="onSearch"
        />
        <Input
          v-model:value="filters.item_id"
          allow-clear
          class="mr-2 w-36"
          placeholder="商品ID"
          @press-enter="onSearch"
        />
        <Button class="mr-2" type="primary" @click="onSearch">查询</Button>
        <Button @click="onReset">重置</Button>
      </template>
    </Grid>
  </Page>
</template>
