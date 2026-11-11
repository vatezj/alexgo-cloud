<script lang="ts" setup>
import type { DictData, DictType } from '#/api/admin';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import { Button, message, Modal as AntdModal } from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  createDictData,
  createDictType,
  deleteDictData,
  deleteDictType,
  listDictDatas,
  listDictTypes,
} from '#/api/admin';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'SystemDictPage' });

/** 上表：字典类型。行操作列的「字典数据」按钮选中该行，驱动下表刷新。 */
const typeGridOptions: VxeGridProps<DictType> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'type', title: '字典类型', minWidth: 160 },
    { field: 'name', title: '名称', minWidth: 140 },
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
      width: 190,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        const rows = await listDictTypes();
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [TypeGrid, typeGridApi] = useVbenVxeGrid({ gridOptions: typeGridOptions });

/** 当前选中的字典类型；下表的 data query、新建数据的 type_id、动态表题都读它。 */
const selectedType = ref<DictType | null>(null);

/** 下表：字典数据。未选中类型时返回空集，表格空转。 */
const dataGridOptions: VxeGridProps<DictData> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'label', title: '标签', minWidth: 140 },
    { field: 'value', title: '值', minWidth: 120 },
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
      width: 130,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        if (!selectedType.value) return { items: [], total: 0 };
        const rows = await listDictDatas(selectedType.value.id);
        return {
          items: localPage(rows, page.currentPage, page.pageSize),
          total: rows.length,
        };
      },
    },
  },
  toolbarConfig: { custom: true, refresh: true, zoom: true },
};

const [DataGrid, dataGridApi] = useVbenVxeGrid({ gridOptions: dataGridOptions });

function onSelectDictType(row: DictType) {
  selectedType.value = row;
  dataGridApi.reload();
}

const [TypeModal, typeModalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建字典类型',
  onOpenChange(isOpen: boolean) {
    if (isOpen) typeFormApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await typeFormApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [TypeForm, typeFormApi] = useVbenForm({
  handleSubmit: async (values) => {
    await createDictType(
      values as { name: string; status: number; type: string },
    );
    message.success('创建成功');
    typeModalApi.close();
    typeGridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 sys_user_sex' },
      fieldName: 'type',
      label: '字典类型',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 用户性别' },
      fieldName: 'name',
      label: '名称',
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

const [DataModal, dataModalApi] = useVbenModal({
  fullscreenButton: false,
  title: '新建字典数据',
  onOpenChange(isOpen: boolean) {
    if (isOpen) dataFormApi.resetForm();
  },
  onConfirm: async () => {
    try {
      await dataFormApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [DataForm, dataFormApi] = useVbenForm({
  handleSubmit: async (values) => {
    if (!selectedType.value) {
      message.warning('请先在字典类型表中点击「字典数据」');
      return;
    }
    await createDictData({
      ...(values as { label: string; sort: number; status: number; value: string }),
      type_id: selectedType.value.id,
    });
    message.success('创建成功');
    dataModalApi.close();
    dataGridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 男' },
      fieldName: 'label',
      label: '标签',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 1' },
      fieldName: 'value',
      label: '值',
      rules: 'required',
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

function onDeleteType(row: DictType) {
  AntdModal.confirm({
    title: `确认删除字典类型「${row.name}」？其下数据请自行清理。`,
    onOk: async () => {
      try {
        await deleteDictType(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      if (selectedType.value?.id === row.id) selectedType.value = null;
      message.success('删除成功');
      typeGridApi.reload();
      dataGridApi.reload();
    },
  });
}

function onDeleteData(row: DictData) {
  AntdModal.confirm({
    title: `确认删除字典数据「${row.label}」？`,
    onOk: async () => {
      try {
        await deleteDictData(row.id);
      } catch {
        return; // 拦截器已 toast
      }
      message.success('删除成功');
      dataGridApi.reload();
    },
  });
}
</script>

<template>
  <Page auto-content-height>
    <div class="flex h-full flex-col gap-4">
      <div class="h-[45%]">
        <TypeGrid>
          <template #toolbar-tools>
            <Button
              class="mr-2"
              type="primary"
              v-access:code="['system:dict:*']"
              @click="() => typeModalApi.open()"
            >
              新建类型
            </Button>
          </template>
          <template #actions="{ row }">
            <Button
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onSelectDictType(row)"
            >
              字典数据
            </Button>
            <Button
              danger
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onDeleteType(row)"
            >
              删除
            </Button>
          </template>
        </TypeGrid>
      </div>
      <div class="flex-1">
        <DataGrid
          :table-title="
            selectedType
              ? `字典数据：${selectedType.name}（${selectedType.type}）`
              : '字典数据：请先点击上方字典类型的「字典数据」'
          "
        >
          <template #toolbar-tools>
            <Button
              class="mr-2"
              type="primary"
              v-access:code="['system:dict:*']"
              @click="() => dataModalApi.open()"
            >
              新建数据
            </Button>
          </template>
          <template #actions="{ row }">
            <Button
              danger
              size="small"
              type="link"
              v-access:code="['system:dict:*']"
              @click="onDeleteData(row)"
            >
              删除
            </Button>
          </template>
        </DataGrid>
      </div>
    </div>
    <TypeModal>
      <TypeForm />
    </TypeModal>
    <DataModal>
      <DataForm />
    </DataModal>
  </Page>
</template>
