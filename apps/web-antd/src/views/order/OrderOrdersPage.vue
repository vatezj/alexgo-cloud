<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { Order } from '#/api/admin';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createOrder, listOrders } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'OrderOrdersPage' });

const gridOptions: VxeGridProps<Order> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'user_id', title: '用户ID', width: 90 },
    { field: 'product_id', title: '商品ID', width: 90 },
    { field: 'amount', title: '金额', width: 110 },
    {
      field: 'status',
      formatter: ({ cellValue }) => String(cellValue ?? ''),
      title: '状态',
      width: 100,
    },
    { field: 'order_no', title: '订单号', minWidth: 180 },
    {
      field: 'created_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '创建时间',
      width: 180,
    },
    {
      field: 'updated_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '更新时间',
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listOrders();
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
  title: '新建订单',
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
    await createOrder(
      values as {
        amount: number;
        order_no: string;
        product_id: number;
        status: number;
        user_id: number;
      },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'InputNumber',
      componentProps: { min: 1 },
      defaultValue: 1,
      fieldName: 'user_id',
      label: '用户ID',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 1 },
      defaultValue: 1,
      fieldName: 'product_id',
      label: '商品ID',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0, precision: 2 },
      defaultValue: 100,
      fieldName: 'amount',
      label: '金额',
      rules: 'required',
    },
    {
      component: 'InputNumber',
      componentProps: { min: 0 },
      defaultValue: 0,
      fieldName: 'status',
      label: '状态',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 SO20261009001' },
      fieldName: 'order_no',
      label: '订单号',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Button
          class="mr-2"
          type="primary"
          v-access:code="['order:order:*']"
          @click="() => modalApi.open()"
        >
          新建订单
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
