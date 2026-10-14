<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { CodegenDemoMenu } from '#/api/order/codegen_demo_menu/types';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';
import { Page, useVbenDrawer, useVbenModal } from '@vben/common-ui';
import { formatDateTime } from '#/utils/format';
import {
  deleteCodegenDemoMenu,
  listCodegenDemoMenu,
  matchQuery,
} from '#/api/order/codegen_demo_menu/api';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { localPage } from '#/utils/local-page';

import DetailDrawerView from './drawer.vue';
import FormModalView from './form.vue';

defineOptions({ name: 'OrderCodegenDemoMenuPage' });
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
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      // 第二参 = 查询表单值（vxe extends.ts 把最后一次提交的表单值并进参数）。
      query: async ({ page }, formValues: Record<string, any>) => {
        const rows = await listCodegenDemoMenu();
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
        label: '主键',
      },
      {
        component: 'Input',
        fieldName: 'parent_id',
        label: '父级ID',
      },
      {
        component: 'Input',
        fieldName: 'name',
        label: '名称',
      },
      {
        component: 'Input',
        fieldName: 'remark',
        label: '备注',
      },
      {
        component: 'Input',
        fieldName: 'created_at',
        label: '创建时间',
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
    title: `确认删除「${String(row.parent_id ?? '')}」？`,
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
