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
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import {
  createFishgameConfigApi,
  deleteFishgameConfigApi,
  getFishgameConfigsApi,
  getFishgameFishesApi,
  updateFishgameConfigApi,
} from '#/api/fishgame';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });
const typeFilter = ref('');

const fishOptions = ref<Array<{ value: string; label: string }>>([]);

async function loadFishes() {
  const res: any = await getFishgameFishesApi({ page: 1, size: 100, status: 1 });
  fishOptions.value = (res.list ?? []).map((f: any) => ({ value: f.fish_id, label: `${f.name}（${f.fish_id}）` }));
}

async function load() {
  loading.value = true;
  try {
    const res: any = await getFishgameConfigsApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
      ...(typeFilter.value ? { type: typeFilter.value } : {}),
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

// ---- 新建/编辑 ----
const modalOpen = ref(false);
const editingId = ref<number | null>(null);
const saving = ref(false);
const form = reactive({
  type: 'luck',
  period: 'any',
  depth_bias: 0,
  fish_id: '',
  week: '',
  status: 1,
});

function openCreate() {
  editingId.value = null;
  Object.assign(form, { type: 'luck', period: 'any', depth_bias: 0, fish_id: '', week: '', status: 1 });
  modalOpen.value = true;
}

function openEdit(row: any) {
  editingId.value = row.id;
  Object.assign(form, {
    type: row.type,
    period: row.period ?? '',
    depth_bias: row.depth_bias ?? 0,
    fish_id: row.fish_id ?? '',
    week: row.week ?? '',
    status: row.status,
  });
  modalOpen.value = true;
}

async function save() {
  saving.value = true;
  try {
    const payload: Record<string, any> = { type: form.type, status: form.status };
    if (form.type === 'luck') {
      payload.period = form.period;
      payload.depth_bias = form.depth_bias;
    } else {
      payload.fish_id = form.fish_id;
      payload.week = form.week;
    }
    if (editingId.value) {
      await updateFishgameConfigApi(editingId.value, payload);
      message.success('已保存');
    } else {
      await createFishgameConfigApi(payload);
      message.success('已创建');
    }
    modalOpen.value = false;
    await load();
  } finally {
    saving.value = false;
  }
}

function confirmDelete(row: any) {
  Modal.confirm({
    title: '删除配置',
    content: `确认删除 ${row.type} 配置 #${row.id}？`,
    onOk: async () => {
      await deleteFishgameConfigApi(row.id);
      message.success('已删除');
      await load();
    },
  });
}

onMounted(() => {
  load();
  loadFishes();
});
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <Select v-model:value="typeFilter" class="w-40" placeholder="配置类型" @change="applyFilter">
          <SelectOption value="">全部</SelectOption>
          <SelectOption value="luck">luck 运气</SelectOption>
          <SelectOption value="fish_king">fish_king 鱼王</SelectOption>
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
        <TableColumn title="ID" data-index="id" :width="70" />
        <TableColumn title="类型" data-index="type">
          <template #default="{ record }">
            <Tag :color="record.type === 'luck' ? 'green' : 'gold'">{{ record.type }}</Tag>
          </template>
        </TableColumn>
        <TableColumn title="时段" data-index="period" />
        <TableColumn title="depth_bias" data-index="depth_bias" />
        <TableColumn title="鱼王 fish_id" data-index="fish_id" />
        <TableColumn title="周" data-index="week" />
        <TableColumn title="状态" :width="80">
          <template #default="{ record }">
            <Tag :color="record.status === 1 ? 'green' : 'default'">
              {{ record.status === 1 ? '启用' : '停用' }}
            </Tag>
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
        :title="editingId ? '编辑配置' : '新建配置'"
        @cancel="modalOpen = false"
        @ok="save"
      >
        <Form layout="vertical">
          <FormItem label="类型">
            <Select v-model:value="form.type">
              <SelectOption value="luck">luck 运气</SelectOption>
              <SelectOption value="fish_king">fish_king 鱼王</SelectOption>
            </Select>
          </FormItem>
          <template v-if="form.type === 'luck'">
            <FormItem label="时段">
              <Input v-model:value="form.period" placeholder="any / day / dusk / night" />
            </FormItem>
            <FormItem help="0 ~ 0.3" label="depth_bias">
              <InputNumber v-model:value="form.depth_bias" :max="0.3" :min="0" :step="0.05" class="w-full" />
            </FormItem>
          </template>
          <template v-else>
            <FormItem label="鱼王 fish_id" required>
              <Select v-model:value="form.fish_id" :options="fishOptions" placeholder="选择启用的鱼" show-search />
            </FormItem>
            <FormItem help="ISO 周，如 2026-W43" label="周" required>
              <Input v-model:value="form.week" placeholder="2026-W43" />
            </FormItem>
          </template>
          <FormItem label="状态">
            <Select v-model:value="form.status">
              <SelectOption :value="1">启用</SelectOption>
              <SelectOption :value="0">停用</SelectOption>
            </Select>
          </FormItem>
        </Form>
      </Modal>
    </div>
  </Page>
</template>
