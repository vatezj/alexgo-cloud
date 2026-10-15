<script lang="ts" setup>
import type { Tenant, TenantPayload } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createTenant,
  deleteTenant,
  listTenants,
  updateTenant,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemTenantsPage' });

const gridOptions: VxeGridProps<Tenant> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'name', title: '名称', minWidth: 140 },
    {
      field: 'domain',
      formatter: ({ cellValue }) => cellValue || '-',
      title: '域名',
      minWidth: 150,
    },
    {
      field: 'status',
      formatter: ({ cellValue }) => (Number(cellValue) === 1 ? '启用' : '停用'),
      title: '状态',
      width: 80,
    },
    {
      field: 'account_limit',
      formatter: ({ cellValue }) =>
        Number(cellValue) === -1 ? '不限' : String(cellValue),
      title: '账号上限',
      width: 100,
    },
    {
      // 后端 Update 载荷不含 expire_time/package_id → 纯展示列
      field: 'expire_time',
      formatter: ({ cellValue }) =>
        cellValue ? String(cellValue).slice(0, 10) : '不限',
      title: '过期时间',
      width: 110,
    },
    {
      field: 'created_at',
      formatter: ({ cellValue }) =>
        cellValue ? String(cellValue).replace('T', ' ').slice(0, 19) : '-',
      title: '创建时间',
      width: 165,
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
        // controller 一次性返回全量（无 page/size 入参、无 total）：
        // 拉全量 + localPage 本地切页。
        const rows = await listTenants();
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

/** null=新建；非空=正在编辑该 id 的租户 */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建租户',
  onOpenChange(isOpen: boolean) {
    if (isOpen) formApi.resetForm();
    else editingId.value = null;
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
    const payload = {
      ...values,
      // 输入框清空会是 null：Go 侧 null→int 保持零值 0，Update 又不归一
      // 0→-1（只有 Create 归一）→ 统一在提交前兜底 -1（不限）。
      account_limit: Number(values.account_limit ?? -1),
    } as TenantPayload;
    if (editingId.value) {
      await updateTenant(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createTenant(payload);
      message.success('创建成功');
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 acme' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 acme.example.com（可空）' },
      fieldName: 'domain',
      label: '域名',
      defaultValue: '',
    },
    {
      // Create 忽略 status（后端恒 1 启用），编辑时生效
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
    {
      component: 'InputNumber',
      componentProps: { min: -1, placeholder: '-1 表示不限' },
      defaultValue: -1,
      fieldName: 'account_limit',
      label: '账号上限',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Tenant) {
  AntdModal.confirm({
    title: `确认删除租户「${row.name}」？`,
    content: '删除后租户数据保留（软删），仅不再出现在列表中。',
    onOk: async () => {
      try {
        await deleteTenant(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      gridApi.reload();
    },
  });
}

function onCreate() {
  editingId.value = null;
  modalApi.setState({ title: '新建租户' });
  modalApi.open();
}

function onEdit(row: Tenant) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑租户' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  void formApi.setValues({
    account_limit: row.account_limit,
    domain: row.domain,
    name: row.name,
    status: row.status,
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
          v-access:code="['system:tenant:*']"
          @click="onCreate"
        >
          新建租户
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['system:tenant:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['system:tenant:*']"
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
