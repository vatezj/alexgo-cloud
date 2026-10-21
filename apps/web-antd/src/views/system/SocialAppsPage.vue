<script lang="ts" setup>
import type { VxeGridProps } from '#/adapter/vxe-table';
import type { SocialApp, SocialAppPayload } from '#/api/admin';

import { ref } from 'vue';

import { Page, useVbenModal } from '@vben/common-ui';

import {
  Modal as AntdModal,
  Button,
  message,
  Select,
  Tag,
} from 'ant-design-vue';

import { useVbenForm, z } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createSocialApp,
  deleteSocialApp,
  listSocialAppsPage,
  updateSocialApp,
} from '#/api/admin';

defineOptions({ name: 'SocialAppsPage' });

function platformLabel(value: unknown): string {
  if (value === 'wechat_mp') return '微信小程序';
  if (value === 'wechat_web') return '微信网页';
  return value ? String(value) : '-';
}

/** 平台筛选：'' = 全平台（后端 platform=='' 视图） */
const platformFilter = ref('');

const platformFilterOptions = [
  { label: '全部', value: '' },
  { label: '微信小程序', value: 'wechat_mp' },
  { label: '微信网页', value: 'wechat_web' },
];

const gridOptions: VxeGridProps<SocialApp> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    {
      field: 'platform',
      formatter: ({ cellValue }) => platformLabel(cellValue),
      title: '平台',
      width: 120,
    },
    { field: 'appid', title: 'AppID', minWidth: 180 },
    { field: 'status', slots: { default: 'status' }, title: '状态', width: 80 },
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
        // 服务端真分页：resp.Page 信封 data:{list,total,page,size}（size≤100）
        const { list, total } = await listSocialAppsPage({
          page: page.currentPage,
          platform: platformFilter.value,
          size: page.pageSize,
        });
        return { items: list, total };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

/** null=新建；非空=正在编辑该 id 的渠道 */
const editingId = ref<null | number>(null);

function onPlatformChange() {
  // gridApi.query() 即 grid.commitProxy('query')：保留当前页重新查询，
  // ajax.query 回调内读 platformFilter.value 带上筛选
  void gridApi.query();
}

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建渠道',
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
    const payload: SocialAppPayload = {
      appid: values.appid,
      platform: values.platform,
      secret: values.secret,
      status: Number(values.status),
    };
    if (editingId.value) {
      await updateSocialApp(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createSocialApp(payload);
      message.success('创建成功');
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Select',
      componentProps: {
        allowClear: false,
        options: [
          { label: '微信小程序', value: 'wechat_mp' },
          { label: '微信网页', value: 'wechat_web' },
        ],
      },
      defaultValue: 'wechat_mp',
      fieldName: 'platform',
      label: '平台',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '微信后台的 AppID（wx 开头）' },
      fieldName: 'appid',
      label: 'AppID',
      rules: 'required',
    },
    {
      component: 'VbenInputPassword',
      componentProps: { placeholder: '编辑留空=保留原值' },
      defaultValue: '',
      fieldName: 'secret',
      // 新建必填；编辑可留空（=保留库中旧值）→ 按 editingId 动态判定
      rules: z
        .string()
        .refine((value) => editingId.value !== null || value.trim() !== '', {
          message: '新建时 AppSecret 必填',
        }),
      label: 'AppSecret（编辑留空=保留原值）',
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

function onDelete(row: SocialApp) {
  AntdModal.confirm({
    title: `确认删除渠道「${platformLabel(row.platform)} / ${row.appid}」？`,
    onOk: async () => {
      try {
        await deleteSocialApp(row.id);
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
  modalApi.setState({ title: '新建渠道' });
  modalApi.open();
}

function onEdit(row: SocialApp) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑渠道' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  void formApi.setValues({
    appid: row.appid,
    platform: row.platform,
    secret: '', // 留空=保留原值（后端 Update 语义）
    status: row.status,
  });
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Select
          v-model:value="platformFilter"
          class="mr-2 w-40"
          :allow-clear="false"
          :options="platformFilterOptions"
          @change="onPlatformChange"
        />
        <Button
          class="mr-2"
          type="primary"
          v-access:code="'social:app:create'"
          @click="onCreate"
        >
          新建渠道
        </Button>
      </template>
      <template #status="{ row }">
        <Tag :color="row.status === 1 ? 'green' : 'default'">
          {{ row.status === 1 ? '启用' : '停用' }}
        </Tag>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="'social:app:update'"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="'social:app:delete'"
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
