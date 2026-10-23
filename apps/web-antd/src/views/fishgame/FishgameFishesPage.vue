<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Form,
  FormItem,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  SelectOption,
  Switch,
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import {
  createFishgameFishApi,
  deleteFishgameFishApi,
  getFishgameFishesApi,
  updateFishgameFishApi,
} from '#/api/fishgame';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

const filters = ref({ rarity: '', status: '' });

async function load() {
  loading.value = true;
  try {
    const res: any = await getFishgameFishesApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
      ...(filters.value.rarity ? { rarity: filters.value.rarity } : {}),
      ...(filters.value.status !== '' ? { status: filters.value.status } : {}),
    });
    list.value = res.list ?? [];
    total.value = res.total ?? 0;
  } finally {
    loading.value = false;
  }
}

function applyFilter() {
  pagination.value.current = 1;
  load();
}

const rarityMap: Record<string, { color: string; text: string }> = {
  normal: { text: '普通', color: 'default' },
  misc: { text: '杂鱼', color: 'cyan' },
  rare: { text: '稀有', color: 'blue' },
  epic: { text: '史诗', color: 'purple' },
};

// ---- 新建/编辑 ----
const modalOpen = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const form = reactive({
  fish_id: '',
  name: '',
  rarity: 'normal',
  period: 'any',
  depth: 'any',
  flavor: '',
  sort: 1,
  status: 1,
});

function openCreate() {
  editingId.value = null;
  Object.assign(form, {
    fish_id: '',
    name: '',
    rarity: 'normal',
    period: 'any',
    depth: 'any',
    flavor: '',
    sort: 1,
    status: 1,
  });
  modalOpen.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  Object.assign(form, {
    fish_id: row.fish_id,
    name: row.name,
    rarity: row.rarity,
    period: row.period,
    depth: row.depth,
    flavor: row.flavor ?? '',
    sort: row.sort,
    status: row.status,
  });
  modalOpen.value = true;
}

async function save() {
  if (!form.fish_id.trim() || !form.name.trim()) {
    message.warning('fish_id 与名称必填');
    return;
  }
  saving.value = true;
  try {
    const payload = { ...form };
    if (editingId.value) {
      await updateFishgameFishApi(editingId.value, payload);
      message.success('已保存');
    } else {
      await createFishgameFishApi(payload);
      message.success('已创建');
    }
    modalOpen.value = false;
    await load();
  } finally {
    saving.value = false;
  }
}

async function toggleStatus(row: any) {
  await updateFishgameFishApi(row.id, {
    fish_id: row.fish_id,
    name: row.name,
    rarity: row.rarity,
    period: row.period,
    depth: row.depth,
    flavor: row.flavor ?? '',
    sort: row.sort,
    status: row.status === 1 ? 0 : 1,
  });
  message.success(row.status === 1 ? '已停用' : '已启用');
  await load();
}

function confirmDelete(row: any) {
  Modal.confirm({
    title: '删除图鉴条目',
    content: `确认删除 ${row.name}（${row.fish_id}）？被点亮或事件引用时会被拒绝。`,
    onOk: async () => {
      await deleteFishgameFishApi(row.id);
      message.success('已删除');
      await load();
    },
  });
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <Select v-model:value="filters.rarity" class="w-36" placeholder="稀有度" @change="applyFilter">
          <SelectOption value="">全部</SelectOption>
          <SelectOption value="normal">普通</SelectOption>
          <SelectOption value="misc">杂鱼</SelectOption>
          <SelectOption value="rare">稀有</SelectOption>
          <SelectOption value="epic">史诗</SelectOption>
        </Select>
        <Select v-model:value="filters.status" class="w-32" placeholder="状态" @change="applyFilter">
          <SelectOption value="">全部</SelectOption>
          <SelectOption value="1">启用</SelectOption>
          <SelectOption value="0">停用</SelectOption>
        </Select>
        <Button type="primary" @click="load">查询</Button>
        <Button type="primary" @click="openCreate">新建</Button>
      </div>
      <Table
        :data-source="list"
        :loading="loading"
        :pagination="{
          ...pagination,
          total,
          onChange: (p: number, ps: number) => {
            pagination.current = p;
            pagination.pageSize = ps;
            load();
          },
        }"
        row-key="id"
      >
        <TableColumn title="fish_id" data-index="fish_id" />
        <TableColumn title="名称" data-index="name" />
        <TableColumn title="稀有度" data-index="rarity">
          <template #default="{ record }">
            <Tag :color="rarityMap[record.rarity]?.color">
              {{ rarityMap[record.rarity]?.text ?? record.rarity }}
            </Tag>
          </template>
        </TableColumn>
        <TableColumn title="时段" data-index="period" />
        <TableColumn title="水层" data-index="depth" />
        <TableColumn title="排序" data-index="sort" :width="70" />
        <TableColumn title="启用" :width="80">
          <template #default="{ record }">
            <Switch
              :checked="record.status === 1"
              checked-children="启"
              un-checked-children="停"
              @change="toggleStatus(record)"
            />
          </template>
        </TableColumn>
        <TableColumn title="操作" :width="160">
          <template #default="{ record }">
            <Button size="small" type="link" @click="openEdit(record)">编辑</Button>
            <Button danger size="small" type="link" @click="confirmDelete(record)">删除</Button>
          </template>
        </TableColumn>
      </Table>

      <Modal
        :confirm-loading="saving"
        :open="modalOpen"
        :title="editingId ? '编辑图鉴条目' : '新建图鉴条目'"
        @cancel="modalOpen = false"
        @ok="save"
      >
        <Form layout="vertical">
          <FormItem label="fish_id" required>
            <Input v-model:value="form.fish_id" :disabled="!!editingId" placeholder="全局唯一 token" />
          </FormItem>
          <FormItem label="名称" required>
            <Input v-model:value="form.name" />
          </FormItem>
          <FormItem label="稀有度">
            <Select v-model:value="form.rarity">
              <SelectOption value="normal">普通</SelectOption>
              <SelectOption value="misc">杂鱼</SelectOption>
              <SelectOption value="rare">稀有</SelectOption>
              <SelectOption value="epic">史诗</SelectOption>
            </Select>
          </FormItem>
          <FormItem label="时段（逗号分隔）">
            <Input v-model:value="form.period" placeholder="any / day,dusk,night" />
          </FormItem>
          <FormItem label="水层">
            <Select v-model:value="form.depth">
              <SelectOption value="any">any</SelectOption>
              <SelectOption value="shallow">shallow</SelectOption>
              <SelectOption value="mid">mid</SelectOption>
              <SelectOption value="deep">deep</SelectOption>
            </Select>
          </FormItem>
          <FormItem label="图鉴文案">
            <Input v-model:value="form.flavor" />
          </FormItem>
          <FormItem label="排序">
            <InputNumber v-model:value="form.sort" :min="0" class="w-full" />
          </FormItem>
          <FormItem label="启用">
            <Switch v-model:checked="form.status" :checked-value="1" :un-checked-value="0" />
          </FormItem>
        </Form>
      </Modal>
    </div>
  </Page>
</template>
