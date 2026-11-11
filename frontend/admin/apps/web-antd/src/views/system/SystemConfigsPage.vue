<script lang="ts" setup>
import type { SystemConfig } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';
import { useAppConfig } from '@vben/hooks';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createConfig,
  deleteConfig,
  listConfigs,
  updateConfig,
} from '#/api/admin';
import { uploadFile } from '#/api/infra';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemConfigsPage' });

/** 分享卡参数键前缀：命中的参数用可视化编辑器（标题 + 图片），其余走裸 value 文本。 */
const SHARE_PREFIX = 'scorecard.share.';

const isShareKey = (key?: unknown): boolean =>
  typeof key === 'string' && key.startsWith(SHARE_PREFIX);

/**
 * 相对 public_url → 绝对地址（微信分享图必须完整 URL）。
 * API 地址为绝对域名时取其源；否则回退当前页面源（dev 走 vite 代理、
 * 生产后台与网关同域时均正确）。已是 http(s) 的外链（如七牛）原样保留。
 */
const { apiURL } = useAppConfig(import.meta.env, import.meta.env.PROD);
function toAbsolutePublicURL(path: string): string {
  if (!path || /^https?:\/\//i.test(path)) return path;
  try {
    if (/^https?:\/\//i.test(apiURL)) return new URL(path, apiURL).toString();
  } catch {
    // apiURL 非法时走页面源兜底
  }
  return `${window.location.origin}${path}`;
}

const gridOptions: VxeGridProps<SystemConfig> = {
  columns: [
    { field: 'id', title: 'ID', width: 80 },
    {
      field: 'key',
      title: '参数键',
      minWidth: 180,
      formatter: ({ cellValue, row }) =>
        isShareKey(row.key) ? `${cellValue}（分享卡）` : cellValue,
    },
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

/** null=新建；非空=正在编辑该 id 的参数 */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建参数',
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
    const key = String(values.key ?? '').trim();
    // 分享卡：value 由可视化字段拼 JSON；普通参数：裸 value 原样存
    const value = isShareKey(key)
      ? JSON.stringify({
          title: String(values.title ?? ''),
          imageUrl: String(values.imageUrl ?? ''),
        })
      : String(values.value ?? '');
    const payload = {
      key,
      name: String(values.name ?? ''),
      remark: String(values.remark ?? ''),
      value,
    };
    if (editingId.value) {
      await updateConfig({ id: editingId.value, ...payload });
      message.success('保存成功');
    } else {
      await createConfig(payload);
      message.success('创建成功');
    }
    modalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: {
        placeholder: '如 site.name；分享卡固定 scorecard.share.{index|simulator|salary}',
      },
      dependencies: {
        // 编辑禁改键（分享卡键被改会断链，普通参数同理防误删语义）
        disabled: () => editingId.value !== null,
        triggerFields: ['key'],
      },
      fieldName: 'key',
      label: '参数键',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 站点名称 / 分享卡 · 首页' },
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
    // ---- 分享卡可视化编辑（key 命中前缀才渲染）----
    {
      component: 'Input',
      componentProps: {
        placeholder: '转发卡片标题，如：登录后开房记账 · 好友互通',
      },
      dependencies: {
        if: (vals) => isShareKey(vals.key),
        triggerFields: ['key'],
      },
      fieldName: 'title',
      label: '分享标题',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: {
        placeholder: '图片 URL；留空 = 微信回退当前页截图',
      },
      dependencies: {
        if: (vals) => isShareKey(vals.key),
        triggerFields: ['key'],
      },
      fieldName: 'imageUrl',
      label: '分享图片',
    },
    {
      component: 'Upload',
      componentProps: {
        // customRequest 接管：上传后把绝对 public_url 回填「分享图片」输入框，
        // 列表缩略图由适配器维护（编辑时用回填 fileList 展示现有图）。
        customRequest: async ({
          file,
          onError,
          onSuccess,
        }: {
          file: File;
          onError: (e: Error) => void;
          onSuccess: (data: unknown) => void;
        }) => {
          try {
            const meta = await uploadFile(file, {
              bizType: 'share',
              public: true,
            });
            const abs = toAbsolutePublicURL(meta.public_url);
            void formApi.setValues({ imageUrl: abs });
            onSuccess({ ...meta, url: abs });
          } catch (e) {
            onError(e instanceof Error ? e : new Error(String(e)));
          }
        },
        fileList: [],
        listType: 'picture-card',
        maxCount: 1,
        multiple: false,
        accept: 'image/*',
        placeholder: '上传图片',
      },
      dependencies: {
        if: (vals) => isShareKey(vals.key),
        triggerFields: ['key'],
      },
      fieldName: 'imageUpload',
      label: '图片上传',
    },
    // ---- 普通参数裸值编辑（非分享卡才渲染）----
    {
      component: 'Textarea',
      componentProps: { placeholder: '参数值', rows: 4 },
      dependencies: {
        if: (vals) => !isShareKey(vals.key),
        triggerFields: ['key'],
      },
      fieldName: 'value',
      label: '参数值',
      rules: 'required',
    },
  ],
  showDefaultActions: false,
});

function onCreate() {
  editingId.value = null;
  modalApi.setState({ title: '新建参数' });
  modalApi.open();
}

function onEdit(row: SystemConfig) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑参数' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  if (isShareKey(row.key)) {
    let title = '';
    let imageUrl = '';
    try {
      const parsed = JSON.parse(row.value) as { title?: string; imageUrl?: string };
      title = parsed.title ?? '';
      imageUrl = parsed.imageUrl ?? '';
    } catch {
      // 非法 JSON：留空让用户重填（提交时会写出合法 JSON）
    }
    void formApi.setValues({
      key: row.key,
      name: row.name,
      remark: row.remark ?? '',
      title,
      imageUrl,
      imageUpload: imageUrl
        ? [{ uid: '-1', name: '当前分享图', status: 'done', url: imageUrl }]
        : [],
    });
  } else {
    void formApi.setValues({
      key: row.key,
      name: row.name,
      remark: row.remark ?? '',
      value: row.value,
    });
  }
}

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
          @click="onCreate"
        >
          新建参数
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['system:config:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
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
