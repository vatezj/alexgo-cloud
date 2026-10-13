<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { NoticeItem } from '#/api/admin';

import { ref } from 'vue';
import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  listMyNotices,
  markAllNoticesRead,
  markNoticeRead,
} from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemMyNoticesPage' });

const gridOptions: VxeGridProps<NoticeItem> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'title', slots: { default: 'title' }, title: '标题', minWidth: 220 },
    {
      field: 'read_status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '已读' : '未读'),
      title: '状态',
      width: 80,
    },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 150,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listMyNotices();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  footer: false, // 纯查看弹窗，隐藏默认底部（与 codegen 预览同款）
  title: '公告详情',
});

const current = ref<NoticeItem | null>(null);

// 打开详情；未读则顺手标记已读（失败只 toast，弹窗照开——看公告不该被标记失败挡住）
async function openView(row: NoticeItem) {
  current.value = row;
  modalApi.open();
  if (row.read_status !== 1) {
    try {
      await markNoticeRead(row.id);
    } catch {
      return; // 拦截器已 toast
    }
    gridApi.reload();
  }
}

async function onMarkRead(row: NoticeItem) {
  try {
    await markNoticeRead(row.id);
  } catch {
    return; // 拦截器已 toast
  }
  message.success('已标记为已读');
  gridApi.reload();
}

async function onReadAll() {
  try {
    await markAllNoticesRead();
  } catch {
    return; // 拦截器已 toast
  }
  message.success('已全部标记为已读');
  gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button class="mr-2" type="primary" @click="onReadAll">
          全部已读
        </Button>
      </template>
      <template #title="{ row }">
        <Button size="small" type="link" @click="openView(row)">
          {{ row.title }}
        </Button>
      </template>
      <template #actions="{ row }">
        <Button size="small" type="link" @click="openView(row)">
          查看
        </Button>
        <Button
          v-if="row.read_status !== 1"
          size="small"
          type="link"
          @click="onMarkRead(row)"
        >
          标记已读
        </Button>
      </template>
    </Grid>
    <Modal>
      <div class="mb-2 text-base font-medium">
        {{ current?.title }}
      </div>
      <div class="mb-3 text-xs text-gray-400">
        {{ current ? formatDateTime(current.created_at) : '' }}
      </div>
      <div class="whitespace-pre-wrap">{{ current?.content }}</div>
    </Modal>
  </Page>
</template>
