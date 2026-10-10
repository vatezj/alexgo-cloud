<script lang="ts" setup>
import type { SystemConfig } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createConfig, deleteConfig, listConfigs } from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemConfigsPage' });

const gridOptions: VxeGridProps<SystemConfig> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'key', title: '参数键', minWidth: 180 },
    { field: 'value', title: '参数值', minWidth: 200 },
    { field: 'name', title: '名称', minWidth: 140 },
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
        const rows = await listConfigs();
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
  title: '新建参数',
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
    await createConfig(
      values as { key: string; name: string; remark: string; value: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 site.name' },
      fieldName: 'key',
      label: '参数键',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 alexGo-cloud' },
      fieldName: 'value',
      label: '参数值',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 站点名称' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: { placeholder: '备注（可空）' },
      defaultValue: '',
      fieldName: 'remark',
      label: '备注',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: SystemConfig) {
  AntdModal.confirm({
    title: `确认删除参数「${row.key}」？`,
    onOk: async () => {
      try {
        await deleteConfig(row.id);
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
          v-access:code="['system:config:*']"
          @click="() => modalApi.open()"
        >
          新建参数
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:config:*']"
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
