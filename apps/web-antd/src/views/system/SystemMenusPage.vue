<script lang="ts" setup>
import type { Menu } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createMenu, deleteMenu, listMenus } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemMenusPage' });

const gridOptions: VxeGridProps<Menu> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'parent_id', title: '父ID', width: 80 },
    {
      field: 'type',
      formatter: ({ cellValue }) =>
        cellValue === 'dir' ? '目录' : cellValue === 'button' ? '按钮' : '菜单',
      title: '类型',
      width: 80,
    },
    { field: 'name', title: '名称', minWidth: 140 },
    { field: 'path', title: '路径', minWidth: 160 },
    { field: 'permission', title: '权限', minWidth: 150 },
    { field: 'sort', title: '排序', width: 70 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listMenus();
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
  title: '新建菜单',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createMenu(
      values as {
        component: string;
        icon: string;
        name: string;
        parent_id: number;
        path: string;
        permission: string;
        sort: number;
        status: number;
        type: string;
      },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 0,
      fieldName: 'parent_id',
      label: '父ID（0=根）',
    },
    {
      component: 'Select',
      componentProps: {
        options: [
          { label: '目录', value: 'dir' },
          { label: '菜单', value: 'menu' },
          { label: '按钮', value: 'button' },
        ],
      },
      defaultValue: 'menu',
      fieldName: 'type',
      label: '类型',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如导出日志' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 /system/export' },
      fieldName: 'path',
      label: '路径',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 views/system/SystemExportPage（按钮留空）' },
      fieldName: 'component',
      label: '组件路径',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 lucide:file-down' },
      fieldName: 'icon',
      label: '图标',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 system:log:export' },
      fieldName: 'permission',
      label: '权限串',
      defaultValue: '',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 1,
      fieldName: 'sort',
      label: '排序',
    },
    {
      component: 'RadioGroup',
      componentProps: {
        options: [
          { label: '启用', value: 1 },
          { label: '停用', value: 0 },
        ],
      },
      defaultValue: 1,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Menu) {
  AntdModal.confirm({
    title: `确认删除菜单「${row.name}」？`,
    onOk: async () => {
      try {
        await deleteMenu(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['system:menu:*']"
          @click="() => modalApi.open()"
        >
          新建菜单
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:menu:*']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
