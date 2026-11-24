<script lang="ts" setup>
import type { TaokoulingChannel } from '#/api/taokouling';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { ref } from 'vue';

import { Page, useVbenModal } from '@vben/common-ui';

import {
  Button,
  Input,
  message,
  Popconfirm,
  Switch,
} from 'ant-design-vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createChannelApi,
  deleteChannelApi,
  listChannelsApi,
  updateChannelApi,
} from '#/api/taokouling';
import { formatDateTime } from '#/utils/format';

defineOptions({ name: 'TaokoulingChannelsPage' });

/** name 筛选（可选，模糊；空 = 不过滤） */
const nameFilter = ref('');

const gridOptions: VxeGridProps<TaokoulingChannel> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'name', title: '名称', minWidth: 130 },
    { field: 'partner', title: 'partner', minWidth: 100 },
    { field: 'host', title: 'host', minWidth: 140 },
    { field: 'pid', title: 'pid', minWidth: 110 },
    { field: 'apikey', title: 'apikey', minWidth: 150 },
    { field: 'tb_name', title: 'tb_name', minWidth: 130 },
    { field: 'status', slots: { default: 'status' }, title: '状态', width: 80 },
    {
      field: 'updated_at',
      formatter: ({ cellValue }) => formatDateTime(cellValue),
      title: '更新时间',
      width: 170,
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
        // 服务端真分页：resp.Page 信封 data:{list,total,page,size}（size≤100）
        const { list, total } = await listChannelsApi({
          page: page.currentPage,
          size: page.pageSize,
          ...(nameFilter.value.trim() ? { name: nameFilter.value.trim() } : {}),
        });
        return { items: list, total };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

function onSearch() {
  // reload 重置到第 1 页再查（query 保留当前页）
  void gridApi.reload();
}

/** null=新建；非空=正在编辑该 id 的渠道 */
const editingId = ref<null | number>(null);

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
    const payload: Partial<TaokoulingChannel> = {
      apikey: String(values.apikey ?? ''),
      host: String(values.host ?? ''),
      name: String(values.name ?? ''),
      partner: String(values.partner ?? ''),
      pid: String(values.pid ?? ''),
      status: Number(values.status ?? 1),
      tb_name: String(values.tb_name ?? ''),
    };
    if (editingId.value) {
      await updateChannelApi(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createChannelApi(payload);
      message.success('创建成功');
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 默认渠道' },
      fieldName: 'name',
      label: '名称',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 kp（空 = 不匹配）' },
      defaultValue: '',
      fieldName: 'partner',
      help: '空 = 不匹配',
      label: 'partner',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 song.vate.ren（空 = 不匹配）' },
      defaultValue: '',
      fieldName: 'host',
      help: '空 = 不匹配',
      label: 'host',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 mm_123_456' },
      fieldName: 'pid',
      label: 'pid',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '淘宝客 APIKey' },
      fieldName: 'apikey',
      label: 'apikey',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 mm_xxx_yyy' },
      fieldName: 'tb_name',
      label: 'tb_name',
      rules: 'required',
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

function onCreate() {
  editingId.value = null;
  modalApi.setState({ title: '新建渠道' });
  modalApi.open();
}

function onEdit(row: TaokoulingChannel) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑渠道' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证（同 Oauth2ClientsPage）
  void formApi.setValues({
    apikey: row.apikey,
    host: row.host,
    name: row.name,
    partner: row.partner,
    pid: row.pid,
    status: row.status,
    tb_name: row.tb_name,
  });
}

/**
 * status 切换走更新接口：后端 UpdateChannel 全量校验（pid/apikey/tb_name 必填
 * + (partner,host) 查重 exclude 自身），故须带整行字段而非仅 status。
 */
async function onStatusToggle(row: TaokoulingChannel, checked: unknown) {
  try {
    await updateChannelApi(row.id, {
      apikey: row.apikey,
      host: row.host,
      name: row.name,
      partner: row.partner,
      pid: row.pid,
      status: checked ? 1 : 0,
      tb_name: row.tb_name,
    });
    message.success('状态已更新');
  } catch {
    // 拦截器已 toast
  } finally {
    gridApi.reload();
  }
}

async function onDelete(row: TaokoulingChannel) {
  try {
    await deleteChannelApi(row.id);
  } catch {
    return; // 拦截器已 toast（默认渠道后端拒）
  }
  message.success('删除成功');
  gridApi.reload();
}
</script>

<template>
  <Page auto-content-height>
    <Grid>
      <template #toolbar-tools>
        <Input
          v-model:value="nameFilter"
          allow-clear
          class="mr-2 w-44"
          placeholder="渠道名称"
          @press-enter="onSearch"
        />
        <Button class="mr-2" type="primary" @click="onSearch">查询</Button>
        <Button type="primary" @click="onCreate">新建渠道</Button>
      </template>
      <template #status="{ row }">
        <Switch
          :checked="row.status === 1"
          size="small"
          @change="onStatusToggle(row, $event)"
        />
      </template>
      <template #actions="{ row }">
        <Button size="small" type="link" @click="onEdit(row)">编辑</Button>
        <Popconfirm title="确认删除该渠道？" @confirm="onDelete(row)">
          <Button danger size="small" type="link">删除</Button>
        </Popconfirm>
      </template>
    </Grid>
    <Modal>
      <Form />
    </Modal>
  </Page>
</template>
