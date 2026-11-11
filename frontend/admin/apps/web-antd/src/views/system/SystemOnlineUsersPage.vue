<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Input,
  message,
  Modal,
  Popconfirm,
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import {
  impersonateApi,
  kickOnlineApi,
  listMemberUsersApi,
  listOnlineApi,
} from '#/api/admin';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });

async function load() {
  loading.value = true;
  try {
    const res = await listOnlineApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
    });
    list.value = (res as any).list ?? res;
    total.value = (res as any).total ?? 0;
  } finally {
    loading.value = false;
  }
}

async function onKick(row: any) {
  await kickOnlineApi(row.id);
  message.success('已踢出，该会话立即失效');
  await load();
}

// ---- 代登录 modal ----
const impOpen = ref(false);
const impLoading = ref(false);
const memberKeyword = ref('');
const memberList = ref<any[]>([]);
const selectedMember = ref<any>(null);

async function searchMembers() {
  impLoading.value = true;
  try {
    const res = await listMemberUsersApi({ page: 1, size: 50 });
    const rows = (res as any).list ?? [];
    const kw = memberKeyword.value.trim().toLowerCase();
    memberList.value = kw
      ? rows.filter(
          (m: any) =>
            String(m.nickname ?? '').toLowerCase().includes(kw) ||
            String(m.mobile ?? '').includes(kw),
        )
      : rows;
  } finally {
    impLoading.value = false;
  }
}

function openImpersonate() {
  selectedMember.value = null;
  memberKeyword.value = '';
  memberList.value = [];
  impOpen.value = true;
  searchMembers();
}

async function confirmImpersonate() {
  if (!selectedMember.value) {
    message.warning('先选择目标会员');
    return;
  }
  // token 不回显、不落盘（spec §4：仅签发+审计；浏览器身份切换留后续小票）
  await impersonateApi(selectedMember.value.id);
  message.success('代登录成功，会话已生效（可在登录日志审计）');
  impOpen.value = false;
  await load();
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <Button type="primary" :loading="loading" @click="load">刷新</Button>
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
        <TableColumn title="ID" data-index="id" :width="80" />
        <TableColumn title="用户名" data-index="username">
          <template #default="{ record }">
            {{ record.username || '-' }}
          </template>
        </TableColumn>
        <TableColumn title="用户ID" data-index="user_id" :width="90" />
        <TableColumn title="客户端" data-index="client_id">
          <template #default="{ record }">
            <Tag v-if="record.is_impersonated" color="orange">代登录</Tag>
            <template v-else>{{ record.client_id || '-' }}</template>
          </template>
        </TableColumn>
        <TableColumn title="签发时间" data-index="issued_at" />
        <TableColumn title="过期时间" data-index="expires_at" />
        <TableColumn title="操作" :width="180">
          <template #default="{ record }">
            <Popconfirm
              title="确认踢出该会话？踢出后立即失效。"
              @confirm="onKick(record)"
            >
              <Button
                v-access:code="['system:online:kick']"
                danger
                size="small"
              >
                踢出
              </Button>
            </Popconfirm>
            <Button
              v-access:code="['system:online:impersonate']"
              class="ml-2"
              size="small"
              @click="openImpersonate"
            >
              代登录
            </Button>
          </template>
        </TableColumn>
      </Table>

      <Modal
        v-model:open="impOpen"
        title="代登录到会员"
        @ok="confirmImpersonate"
      >
        <Input
          v-model:value="memberKeyword"
          allow-clear
          class="mb-3"
          placeholder="按昵称/手机过滤（前 50 条）"
          @press-enter="searchMembers"
        />
        <Table
          :columns="[
            { title: 'ID', dataIndex: 'id', width: 80 },
            { title: '昵称', dataIndex: 'nickname' },
            { title: '手机', dataIndex: 'mobile' },
          ]"
          :data-source="memberList"
          :loading="impLoading"
          :pagination="false"
          row-key="id"
          size="small"
          :row-selection="{
            type: 'radio',
            onSelect: (r: any) => {
              selectedMember = r;
            },
          }"
        />
      </Modal>
    </div>
  </Page>
</template>
