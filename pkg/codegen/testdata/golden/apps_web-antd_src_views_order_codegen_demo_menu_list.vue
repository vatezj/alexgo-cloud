<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { CodegenDemoMenu } from '#/api/order/codegen_demo_menu/types';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';
import { Page, useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { formatDateTime } from '#/utils/format';
import {
  deleteCodegenDemoMenu,
  listCodegenDemoMenu,
} from '#/api/order/codegen_demo_menu/api';
import { useVbenVxeGrid } from '#/adapter/vxe-table';

import DetailDrawerView from './drawer.vue';
import FormModalView from './form.vue';

defineOptions({ name: 'OrderCodegenDemoMenuPage' });

// 树表显示名固定 name 列（spec §10.1 约定列，删除确认文案用）。
const gridOptions: VxeGridProps<CodegenDemoMenu> = {
  columns: [
    {
      field: 'id',
      title: '主键',
      width: 80,
    },
    {
      field: 'parent_id',
      title: '父级ID',
      minWidth: 120,
    },
    {
      field: 'name',
      title: '名称',
      minWidth: 120,
    },
    {
      field: 'remark',
      title: '备注',
      minWidth: 120,
    },
    {
      field: 'created_at',
      title: '创建时间',
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
  // 树形交互（spec §10.4，房规 SystemMenusPage 原样）：平铺 parent_id 由 vxe 转树、
  // 默认全展开；关分页与搜索——过滤/分页都会破坏树形完整性。
  treeConfig: { expandAll: true, parentField: 'parent_id', rowField: 'id', transform: true },
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async () => {
        const rows = await listCodegenDemoMenu();
        return { items: rows, total: rows.length };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

// 无 formOptions：树表不提供搜索表单（spec §10.4）。
const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

const [FormModal, formModalApi] = useVbenModal({
  connectedComponent: FormModalView,
});

const [DetailDrawer, detailDrawerApi] = useVbenDrawer({
  connectedComponent: DetailDrawerView,
});

function onCreate() {
  formModalApi.setState({ title: '新建CodegenDemoMenu' });
  formModalApi.setData({}); // 清残留：防子组件把上次的旧数据当编辑
  formModalApi.open();
}

function onEdit(row: CodegenDemoMenu) {
  formModalApi.setState({ title: '编辑CodegenDemoMenu' });
  formModalApi.setData(row);
  formModalApi.open();
}

function onDetail(row: CodegenDemoMenu) {
  detailDrawerApi.setData(row);
  detailDrawerApi.open();
}

function onDelete(row: CodegenDemoMenu) {
  AntdModal.confirm({
    title: `确认删除「${String(row.name ?? '')}」？`,
    content: '删除后不可恢复。',
    onOk: async () => {
      try {
        await deleteCodegenDemoMenu(row.id);
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
          v-access:code="['order:codegen_demo_menu:*']"
          @click="onCreate"
        >
          新建
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['order:codegen_demo_menu:*']"
          @click="onDetail(row)"
        >
          详情
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['order:codegen_demo_menu:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['order:codegen_demo_menu:*']"
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
