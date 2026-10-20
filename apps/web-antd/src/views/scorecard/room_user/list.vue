<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { RoomUser } from '#/api/scorecard/room_user/types';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';
import { Page, useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { formatDateTime } from '#/utils/format';
import {
  deleteRoomUser,
  listRoomUser,
  matchQuery,
} from '#/api/scorecard/room_user/api';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { localPage } from '#/utils/local-page';

import DetailDrawerView from './drawer.vue';
import FormModalView from './form.vue';

defineOptions({ name: 'ScorecardRoomUserPage' });
const gridOptions: VxeGridProps<RoomUser> = {
  columns: [
    {
      field: 'id',
      title: 'id',
      width: 80,
    },
    {
      field: 'name',
      title: '名称',
      minWidth: 120,
    },
    {
      field: 'status',
      title: '状态 1=启用 0=停用',
      minWidth: 120,
    },
    {
      field: 'created_at',
      title: '创建时间',
      minWidth: 120,
      formatter: ({ cellValue }) => formatDateTime(cellValue),
    },
    {
      field: 'updated_at',
      title: '更新时间',
      minWidth: 120,
      formatter: ({ cellValue }) => formatDateTime(cellValue),
    },
    {
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      // 第二参 = 查询表单值（vxe extends.ts 把最后一次提交的表单值并进参数）。
      query: async ({ page }, formValues: Record<string, any>) => {
        const rows = await listRoomUser();
        const filtered = matchQuery(rows, formValues ?? {});
        return {
          items: localPage(filtered, page.currentPage, page.pageSize),
          total: filtered.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({
  formOptions: {
    schema: [
      {
        component: 'Input',
        fieldName: 'id',
        label: 'id',
      },
      {
        component: 'Input',
        fieldName: 'name',
        label: '名称',
      },
      {
        component: 'Input',
        fieldName: 'status',
        label: '状态 1=启用 0=停用',
      },
      {
        component: 'Input',
        fieldName: 'created_at',
        label: '创建时间',
      },
      {
        component: 'Input',
        fieldName: 'updated_at',
        label: '更新时间',
      },
    ],
  },
  gridOptions,
});

const [FormModal, formModalApi] = useVbenModal({
  connectedComponent: FormModalView,
});

const [DetailDrawer, detailDrawerApi] = useVbenDrawer({
  connectedComponent: DetailDrawerView,
});

function onCreate() {
  formModalApi.setState({ title: '新建RoomUser' });
  formModalApi.setData({}); // 清残留：防子组件把上次的旧数据当编辑
  formModalApi.open();
}

function onEdit(row: RoomUser) {
  formModalApi.setState({ title: '编辑RoomUser' });
  formModalApi.setData(row);
  formModalApi.open();
}

function onDetail(row: RoomUser) {
  detailDrawerApi.setData(row);
  detailDrawerApi.open();
}

function onDelete(row: RoomUser) {
  AntdModal.confirm({
    title: `确认删除「${String(row.name ?? '')}」？`,
    content: '删除后不可恢复。',
    onOk: async () => {
      try {
        await deleteRoomUser(row.id);
      } catch {
        return; // 拦截器已 toast，保持确认框
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}

function onSaved() {
  gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['scorecard:room_user:*']"
          @click="onCreate"
        >
          新建
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['scorecard:room_user:*']"
          @click="onDetail(row)"
        >
          详情
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['scorecard:room_user:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['scorecard:room_user:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <FormModal @saved="onSaved" />
    <DetailDrawer />
  </Page>
</template>
