<script lang="ts" setup>
import type { Oauth2Client, Oauth2ClientPayload } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createClient,
  deleteClient,
  listClients,
  updateClient,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'Oauth2ClientsPage' });

const gridOptions: VxeGridProps<Oauth2Client> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'client_id', title: 'Client ID', minWidth: 160 },
    { field: 'client_name', title: '名称', minWidth: 140 },
    { field: 'grant_types', title: '授权模式', minWidth: 200 },
    { field: 'account_types', title: '账号类型', width: 130 },
    {
      field: 'activity',
      formatter: ({ cellValue }) =>
        Number(cellValue) === 1 ? '启用' : '停用',
      title: '状态',
      width: 80,
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
        // {data,total} 被 requestClient 拦截器解包后 total 丢失：
        // 一次拉全量（controller size 上限 200）+ 本地切页，对齐 SystemPostsPage。
        const rows = await listClients();
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

/** null=新建；非空=正在编辑该 id 的客户端 */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建客户端',
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
    const payload = values as Oauth2ClientPayload;
    if (editingId.value) {
      await updateClient(editingId.value, payload);
      message.success('保存成功');
    } else {
      const created = await createClient(payload);
      message.success('创建成功');
      if (created.client_secret) {
        // 明文 secret 仅创建响应回显一次（spec §3.1），错过即无法再取
        AntdModal.info({
          title: '客户端密钥（仅显示一次）',
          content: created.client_secret,
        });
      }
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 third-party-crm' },
      fieldName: 'client_id',
      label: 'Client ID',
      rules: 'required',
    },
    {
      component: 'VbenInputPassword',
      componentProps: { placeholder: '新建留空=公开客户端；编辑留空=不改' },
      fieldName: 'client_secret',
      label: '客户端密钥',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 第三方 CRM' },
      fieldName: 'client_name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: 'https://example.com' },
      fieldName: 'website',
      label: '主页',
      defaultValue: '',
    },
    {
      component: 'Textarea',
      componentProps: {
        placeholder:
          '逗号分隔：authorization_code,client_credentials,password,refresh_token',
      },
      fieldName: 'grant_types',
      label: '授权模式',
      rules: 'required',
    },
    {
      component: 'Textarea',
      componentProps: {
        placeholder: '每行一个，精确匹配：http://localhost:5173/callback',
      },
      fieldName: 'redirect_uris',
      label: '回跳白名单',
      defaultValue: '',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '逗号分隔：system,member' },
      fieldName: 'account_types',
      label: '账号类型',
      rules: 'required',
    },
    {
      component: 'Input',
      defaultValue: 'all',
      fieldName: 'scopes',
      label: 'Scope',
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
      fieldName: 'activity',
      label: '状态',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onDelete(row: Oauth2Client) {
  AntdModal.confirm({
    title: `确认删除客户端「${row.client_id}」？`,
    onOk: async () => {
      try {
        await deleteClient(row.id);
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
  modalApi.setState({ title: '新建客户端' });
  modalApi.open();
}

function onEdit(row: Oauth2Client) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑客户端' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  void formApi.setValues({
    account_types: row.account_types,
    activity: row.activity,
    client_id: row.client_id,
    client_secret: '', // 留空=不改（后端 Update 语义）
    client_name: row.client_name,
    grant_types: row.grant_types,
    redirect_uris: row.redirect_uris,
    scopes: row.scopes,
    website: row.website,
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
          v-access:code="['oauth2:client:create']"
          @click="onCreate"
        >
          新建客户端
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['oauth2:client:update']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['oauth2:client:delete']"
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
