<script lang="ts" setup>
import type { Menu, MenuPayload } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import { createMenu, deleteMenu, listMenus, updateMenu } from '#/api/admin';

defineOptions({ name: 'SystemMenusPage' });

const gridOptions: VxeGridProps<Menu> = {
  columns: [
    // 树形节点图标/缩进挂在名称列上
    { field: 'name', title: '名称', minWidth: 200, treeNode: true },
    { field: 'id', title: 'ID', width: 70 },
    {
      field: 'type',
      formatter: ({ cellValue }) =>
        cellValue === 'dir' ? '目录' : cellValue === 'button' ? '按钮' : '菜单',
      title: '类型',
      width: 80,
    },
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
      width: 180,
    },
  ],
  height: 'auto',
  keepSource: true,
  // 树形表：扁平的 parent_id 列表交给 vxe 转成树，默认全部展开、可逐级折叠
  treeConfig: {
    expandAll: true,
    parentField: 'parent_id',
    rowField: 'id',
    transform: true,
  },
  // 树形结构不能按页切分，一次性加载全量菜单
  pagerConfig: { enabled: false },
  proxyConfig: {
    ajax: {
      query: async () => {
        const rows = await listMenus();
        return { items: rows, total: rows.length };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [Grid, gridApi] = useVbenVxeGrid({ gridOptions });

/** null=新建；非空=正在编辑该 id 的菜单 */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建菜单',
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
    const payload = values as MenuPayload;
    if (editingId.value) {
      await updateMenu(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createMenu(payload);
      message.success('创建成功');
    }
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

function onCreate() {
  editingId.value = null;
  modalApi.setState({ title: '新建菜单' });
  modalApi.open();
}

function onEdit(row: Menu) {
  editingId.value = row.id;
  modalApi.setState({ title: '编辑菜单' });
  modalApi.open();
  // open() 同步触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证
  void formApi.setValues({
    component: row.component,
    icon: row.icon,
    name: row.name,
    parent_id: row.parent_id,
    path: row.path,
    permission: row.permission,
    sort: row.sort,
    status: row.status,
    type: row.type,
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
          @click="onCreate"
        >
          新建菜单
        </Button>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['system:menu:*']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
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
