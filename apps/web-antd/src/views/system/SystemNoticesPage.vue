<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { Notice } from '#/api/admin';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createNotice, deleteNotice, listNotices } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemNoticesPage' });

const gridOptions: VxeGridProps<Notice> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'title', title: '标题', minWidth: 220 },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
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
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listNotices();
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
  title: '新建公告',
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
    await createNotice(
      values as { content: string; status: number; title: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '请输入公告标题' },
      fieldName: 'title',
      label: '标题',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: { placeholder: '请输入公告内容', rows: 4 },
      fieldName: 'content',
      label: '内容',
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

function onDelete(row: Notice) {
  AntdModal.confirm({
    title: `确认删除公告「${row.title}」？`,
    onOk: async () => {
      try {
        await deleteNotice(row.id);
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
          v-access:code="['system:notice:*']"
          @click="() => modalApi.open()"
        >
          新建公告
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:notice:*']"
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
