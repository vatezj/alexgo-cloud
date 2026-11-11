<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { OperateLog } from '#/api/admin';

import { Page } from '@vben/common-ui';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { listOperateLogs } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemOperateLogsPage' });

const gridOptions: VxeGridProps<OperateLog> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'tenant_id', title: '租户', width: 80 },
    { field: 'username', title: '用户名', minWidth: 110 },
    { field: 'method', title: '方法', width: 80 },
    { field: 'path', title: '路径', minWidth: 200 },
    { field: 'status', title: 'HTTP状态', width: 100 },
    { field: 'latency_ms', title: '耗时(ms)', width: 100 },
    { field: 'error', title: '错误', minWidth: 150 },
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
        const rows = await listOperateLogs();
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
