<script lang="ts" setup>
import type { User } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message } from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createUser, listUsers, resetUserPassword } from '#/api/admin';
import { formatDateTime } from '#/utils/format';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemUsersPage' });

const gridOptions: VxeGridProps<User> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    { field: 'username', title: '用户名', minWidth: 140 },
    { field: 'nickname', title: '昵称', minWidth: 140 },
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
      field: 'updated_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '更新时间',
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
        const rows = await listUsers();
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
  title: '新建用户',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 后端错误已由拦截器 toast；弹窗保持打开
    }
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createUser(
      values as { nickname: string; password: string; username: string },
    );
    message.success('创建成功');
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '请输入用户名' },
      fieldName: 'username',
      label: '用户名',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '请输入昵称' },
      fieldName: 'nickname',
      label: '昵称',
      rules: 'required',
    },
    {
      component: 'InputPassword',
      componentProps: { placeholder: '请输入初始密码' },
      fieldName: 'password',
      label: '密码',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onResetPassword(row: User) {
  const pwd = window.prompt(`为 ${row.username} 设置新密码`);
  if (!pwd) return;
  resetUserPassword(row.id, pwd)
    .then(() => message.success('密码已重置'))
    .catch(() => {
      // 拦截器已 toast
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
          v-access:code="['system:user:list']"
          @click="() => modalApi.open()"
        >
          新建用户
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['system:user:list']"
          @click="onResetPassword(row)"
        >
          重置密码
        </Button>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
