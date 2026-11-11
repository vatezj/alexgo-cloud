<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { LoginLog } from '#/api/admin';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listLoginLogs } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemLoginLogsPage' });

const gridOptions: VxeGridProps<LoginLog> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'tenant_id', title: '租户', width: 80 },
    { field: 'username', title: '用户名', minWidth: 120 },
    { field: 'user_id', title: '用户ID', width: 90 },
    { field: 'ip', title: 'IP', minWidth: 130 },
    {
      field: 'success',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '成功' : '失败'),
      title: '结果',
      width: 80,
    },
    { field: 'message', title: '信息', minWidth: 160 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '时间',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listLoginLogs();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid] = useVbenVxeGrid({ gridOptions });
</script>

<template>
  <Page auto-content-height>
    <Grid />
  </Page>
</template>
