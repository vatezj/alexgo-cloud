<script lang="ts" setup>
import type { CodegenColumn, CodegenTable } from '#/api/infra';
import type { VxeGridProps } from '#/adapter/vxe-table';

import { Page, useVbenModal } from '@vben/common-ui';

import {
  Button,
  Input,
  InputNumber,
  message,
  Modal as AntdModal,
  Select,
  Switch,
  Table,
  Tag,
  Tooltip,
} from 'ant-design-vue';

import { ref } from 'vue';

import { useVbenForm } from '#/adapter/form';
import { useVbenVxeGrid } from '#/adapter/vxe-table';
import {
  deleteCodegenTable,
  importCodegenTable,
  listCodegenColumns,
  listCodegenTables,
  listDBTables,
  saveCodegenColumns,
  syncCodegenTable,
  updateCodegenTable,
} from '#/api/infra';
import { localPage } from '#/utils/local-page';

defineOptions({ name: 'InfraCodegenPage' });

// pkg/codegen/model/enums.go 的 HTMLType 取值
const HTML_TYPE_OPTIONS = [
  { label: '输入框', value: 'Input' },
  { label: '多行文本', value: 'Textarea' },
  { label: '数字输入', value: 'InputNumber' },
  { label: '下拉选择', value: 'Select' },
  { label: '开关', value: 'Switch' },
  { label: '日期', value: 'DatePicker' },
];

// spec §9.4 / builder：查询操作只支持三种
const QUERY_OP_OPTIONS = [
  { label: '等于 eq', value: 'eq' },
  { label: '模糊 like', value: 'like' },
  { label: '区间 between', value: 'between' },
];

const gridOptions: VxeGridProps<CodegenTable> = {
  columns: [
    { field: 'id', title: 'ID', width: 70 },
    { field: 'table_name', title: '表名', minWidth: 160 },
    {
      field: 'table_comment',
      title: '表注释',
      minWidth: 140,
      formatter: ({ cellValue }) => cellValue || '-',
    },
    { field: 'module', title: '模块', width: 100 },
    { field: 'business_name', title: '业务名', minWidth: 120 },
    { field: 'class_name', title: '类名', minWidth: 130 },
    {
      field: 'template_type',
      title: '模板',
      width: 90,
      formatter: ({ cellValue }) =>
        ({ 1: '单表', 2: '树型', 3: '主子' })[Number(cellValue)] ?? '-',
    },
    {
      field: 'operation',
      fixed: 'right',
      slots: { default: 'actions' },
      title: '操作',
      width: 260,
    },
  ],
  height: 'auto',
  keepSource: true,
  pagerConfig: {},
  proxyConfig: {
    ajax: {
      query: async ({ page }) => {
        // {data,total} 被 requestClient 解包后 total 丢失：拉全量（size 上限
        // 200）+ 本地切页，对齐 Oauth2ClientsPage/SystemPostsPage。
        const rows = await listCodegenTables();
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

// ---- 导入弹窗：下拉选库内表 ----
const dbTables = ref<string[]>([]);
const pickedTable = ref<string>();

const [ImportModal, importModalApi] = useVbenModal({
  title: '导入数据表',
  onOpenChange(isOpen) {
    if (isOpen) {
      pickedTable.value = undefined;
      void loadDBTables();
    }
  },
  onConfirm: async () => {
    if (!pickedTable.value) {
      message.warning('请先选择要导入的表');
      return;
    }
    try {
      await importCodegenTable(pickedTable.value);
    } catch {
      return; // 拦截器已 toast（重复导入/表不存在 → 400），弹窗保持打开
    }
    message.success('导入成功');
    importModalApi.close();
    gridApi.reload();
  },
});

async function loadDBTables() {
  try {
    dbTables.value = await listDBTables();
  } catch {
    // 拦截器已 toast
  }
}

function onImport() {
  importModalApi.open();
}

// ---- 字段配置弹窗：全量编辑用户配置列 ----
const editingTable = ref<CodegenTable | null>(null);
const columnsRows = ref<CodegenColumn[]>([]);
const columnsLoading = ref(false);

const columnGridColumns = [
  { key: 'name', title: '字段', width: 130 },
  { key: 'type', title: '类型', width: 100 },
  { key: 'comment', title: '注释（快照）', width: 150 },
  { key: 'html_type', title: '控件', width: 130 },
  { key: 'list_enable', title: '列表', width: 60 },
  { key: 'form_enable', title: '表单', width: 60 },
  { key: 'query_enable', title: '查询', width: 60 },
  { key: 'query_operation', title: '查询方式', width: 120 },
  { key: 'dict_type', title: '字典类型', width: 130 },
  { key: 'sort_order', title: '排序', width: 80 },
  { key: 'deprecated', title: '状态', width: 80 },
];

const [ColumnsModal, columnsModalApi] = useVbenModal({
  title: '字段配置',
  onOpenChange(isOpen) {
    if (!isOpen) {
      editingTable.value = null;
      columnsRows.value = [];
    }
  },
  onConfirm: async () => {
    if (!editingTable.value) return;
    try {
      await saveCodegenColumns(editingTable.value.id, columnsRows.value);
    } catch {
      return; // 拦截器已 toast
    }
    message.success('保存成功');
    columnsModalApi.close();
  },
});

async function onColumns(row: CodegenTable) {
  editingTable.value = row;
  columnsRows.value = [];
  columnsModalApi.setState({ title: `字段配置 — ${row.table_name}` });
  columnsModalApi.open();
  columnsLoading.value = true;
  try {
    columnsRows.value = await listCodegenColumns(row.id);
  } catch {
    // 拦截器已 toast；空表打开可重试
  } finally {
    columnsLoading.value = false;
  }
}

// ---- 表配置编辑：模块/业务名/类名/备注 ----
const editingId = ref<number | null>(null);

const [EditModal, editModalApi] = useVbenModal({
  title: '编辑表配置',
  onOpenChange(isOpen) {
    if (isOpen) {
      editFormApi.resetForm();
    } else {
      editingId.value = null;
    }
  },
  onConfirm: async () => {
    try {
      await editFormApi.validateAndSubmitForm();
    } catch {
      // 拦截器已 toast，弹窗保持打开
    }
  },
});

const [EditForm, editFormApi] = useVbenForm({
  handleSubmit: async (values) => {
    if (!editingId.value) return;
    try {
      await updateCodegenTable(editingId.value, values);
    } catch {
      return; // 拦截器已 toast
    }
    message.success('保存成功');
    editModalApi.close();
    gridApi.reload();
  },
  schema: [
    {
      component: 'Input',
      componentProps: { placeholder: '如 system / order' },
      fieldName: 'module',
      label: '目标模块',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 user / order' },
      fieldName: 'business_name',
      label: '业务名',
      rules: 'required',
    },
    {
      component: 'Input',
      componentProps: { placeholder: '如 SystemUser' },
      fieldName: 'class_name',
      label: '类名',
      rules: 'required',
    },
    {
      component: 'Textarea',
      fieldName: 'remark',
      label: '备注',
      defaultValue: '',
    },
  ],
  showDefaultActions: false,
});

function onEdit(row: CodegenTable) {
  editingId.value = row.id;
  editModalApi.open();
  // open() 触发 onOpenChange 的 resetForm（其 await getForm 先入队），
  // 随后的 setValues 在其后入队 → 先重置再回填，顺序有保证（同 Oauth2ClientsPage）
  void editFormApi.setValues({
    business_name: row.business_name,
    class_name: row.class_name,
    module: row.module,
    remark: row.remark,
  });
}

// ---- 同步 / 删除 ----
async function onSync(row: CodegenTable) {
  try {
    const res = await syncCodegenTable(row.id);
    message.success(
      `同步完成：新增 ${res.added} · 更新 ${res.updated} · 废弃 ${res.deprecated} · 未变 ${res.unchanged}`,
    );
  } catch {
    return; // 拦截器已 toast
  }
  gridApi.reload();
}

function onDelete(row: CodegenTable) {
  AntdModal.confirm({
    title: `确认删除表配置「${row.table_name}」？`,
    content: '仅删除生成配置快照，不会删除数据库表本身。',
    onOk: async () => {
      try {
        await deleteCodegenTable(row.id);
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
          v-access:code="['infra:codegen:import']"
          @click="onImport"
        >
          导入表
        </Button>
        <Tooltip title="生成代码是 M3 能力，后端尚未实现">
          <Button disabled>生成代码</Button>
        </Tooltip>
      </template>
      <template #actions="{ row }">
        <Button
          size="small"
          type="link"
          v-access:code="['infra:codegen']"
          @click="onColumns(row)"
        >
          字段配置
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['infra:codegen']"
          @click="onEdit(row)"
        >
          编辑
        </Button>
        <Button
          size="small"
          type="link"
          v-access:code="['infra:codegen:sync']"
          @click="onSync(row)"
        >
          同步
        </Button>
        <Button
          danger
          size="small"
          type="link"
          v-access:code="['infra:codegen:delete']"
          @click="onDelete(row)"
        >
          删除
        </Button>
      </template>
    </Grid>

    <!-- 导入：下拉选库内表 -->
    <ImportModal>
      <div class="py-2">
        <div class="mb-2 text-sm text-gray-500">
          选择要导入的数据库表（重复导入会被拒绝）
        </div>
        <Select
          v-model:value="pickedTable"
          :options="dbTables.map((n) => ({ label: n, value: n }))"
          placeholder="输入表名搜索"
          show-search
          style="width: 100%"
        />
      </div>
    </ImportModal>

    <!-- 字段配置：全量编辑用户配置列 -->
    <ColumnsModal>
      <Table
        :columns="columnGridColumns"
        :data-source="columnsRows"
        :loading="columnsLoading"
        :pagination="false"
        :scroll="{ x: 1170, y: 420 }"
        bordered
        row-key="id"
        size="small"
      >
        <template #bodyCell="{ column, record }">
          <template v-if="column.key === 'name'">
            <span class="font-mono">{{ record.name }}</span>
            <Tag v-if="record.is_pk" class="ml-1" color="blue">PK</Tag>
          </template>
          <template v-else-if="column.key === 'type'">
            <span class="font-mono text-xs">{{ record.type }}</span>
          </template>
          <template v-else-if="column.key === 'comment'">
            <!-- 快照字段（§9.4）：由同步从 DB 注释刷新，SaveColumns 不写它，
                 所以只读展示，可编辑的是下面的用户配置列 -->
            <span class="text-xs text-gray-500">
              {{ record.comment || '-' }}
            </span>
          </template>
          <template v-else-if="column.key === 'html_type'">
            <Select
              v-model:value="record.html_type"
              :options="HTML_TYPE_OPTIONS"
              size="small"
            />
          </template>
          <template v-else-if="column.key === 'list_enable'">
            <Switch v-model:checked="record.list_enable" size="small" />
          </template>
          <template v-else-if="column.key === 'form_enable'">
            <Switch v-model:checked="record.form_enable" size="small" />
          </template>
          <template v-else-if="column.key === 'query_enable'">
            <Switch v-model:checked="record.query_enable" size="small" />
          </template>
          <template v-else-if="column.key === 'query_operation'">
            <Select
              v-model:value="record.query_operation"
              :disabled="!record.query_enable"
              :options="QUERY_OP_OPTIONS"
              size="small"
            />
          </template>
          <template v-else-if="column.key === 'dict_type'">
            <Input v-model:value="record.dict_type" size="small" />
          </template>
          <template v-else-if="column.key === 'sort_order'">
            <InputNumber v-model:value="record.sort_order" size="small" />
          </template>
          <template v-else-if="column.key === 'deprecated'">
            <Tag v-if="record.deprecated" color="warning">已废弃</Tag>
            <Tag v-else color="success">在用</Tag>
          </template>
        </template>
      </Table>
    </ColumnsModal>

    <!-- 表配置编辑 -->
    <EditModal>
      <EditForm />
    </EditModal>
  </Page>
</template>
