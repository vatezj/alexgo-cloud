<script setup lang="ts">
import { onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import {
  Button,
  Drawer,
  Empty,
  Input,
  message,
  Spin,
  Table,
  TableColumn,
  Tag,
} from 'ant-design-vue';

import {
  getFishgamePlayerDetailApi,
  getFishgamePlayerRankApi,
  getFishgamePlayersApi,
} from '#/api/fishgame';

const loading = ref(false);
const list = ref<any[]>([]);
const total = ref(0);
const pagination = ref({ current: 1, pageSize: 20 });
const keyword = ref('');

async function load() {
  loading.value = true;
  try {
    const res: any = await getFishgamePlayersApi({
      page: pagination.value.current,
      size: pagination.value.pageSize,
      ...(keyword.value.trim() ? { keyword: keyword.value.trim() } : {}),
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

// ---- 详情抽屉 ----
const drawerOpen = ref(false);
const detailLoading = ref(false);
const detail = ref<any>(null);
const lit = ref<Array<{ fish_id: string; count: number }>>([]);
const events = ref<any[]>([]);
const rankText = ref('');

async function openDetail(row: any) {
  drawerOpen.value = true;
  detailLoading.value = true;
  detail.value = null;
  lit.value = [];
  events.value = [];
  rankText.value = '';
  try {
    const d: any = await getFishgamePlayerDetailApi(row.id);
    detail.value = d.player;
    lit.value = d.lit ?? [];
    events.value = d.events ?? [];
    try {
      const r: any = await getFishgamePlayerRankApi(row.id);
      rankText.value = `第 ${r.position} 名 / 共 ${r.total} 人（当前榜口径）`;
    } catch {
      rankText.value = '榜单位次不可用';
    }
  } catch {
    message.error('详情加载失败');
  } finally {
    detailLoading.value = false;
  }
}

function t2str(t: number) {
  return new Date(t * 1000).toLocaleString();
}

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <div class="p-4">
      <div class="mb-3 flex gap-2">
        <Input
          v-model:value="keyword"
          allow-clear
          class="w-64"
          placeholder="openid / 昵称 模糊搜索"
          @press-enter="applyFilter"
        />
        <Button type="primary" @click="applyFilter">搜索</Button>
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
        <TableColumn title="openid" data-index="openid" />
        <TableColumn title="昵称" data-index="nick" />
        <TableColumn title="点亮数" data-index="dex" :width="90" />
        <TableColumn title="注册时间" data-index="created_at" />
        <TableColumn title="最近登录" data-index="last_login_at" />
        <TableColumn title="操作" :width="90">
          <template #default="{ record }">
            <Button size="small" type="link" @click="openDetail(record)">详情</Button>
          </template>
        </TableColumn>
      </Table>

      <Drawer v-model:open="drawerOpen" :width="560" title="玩家详情">
        <Spin :spinning="detailLoading">
          <template v-if="detail">
            <div class="mb-4 space-y-1">
              <div><b>ID：</b>{{ detail.id }}</div>
              <div><b>openid：</b>{{ detail.openid }}</div>
              <div><b>昵称：</b>{{ detail.nick }}</div>
              <div><b>注册：</b>{{ detail.created_at }}</div>
              <div><b>最近登录：</b>{{ detail.last_login_at || '—' }}</div>
              <div><Tag color="blue">{{ rankText || '榜单位次…' }}</Tag></div>
            </div>

            <h3 class="mb-2 font-semibold">点亮明细（{{ lit.length }}）</h3>
            <Table
              v-if="lit.length"
              :data-source="lit"
              :pagination="false"
              class="mb-4"
              row-key="fish_id"
              size="small"
            >
              <TableColumn title="fish_id" data-index="fish_id" />
              <TableColumn title="次数" data-index="count" :width="80" />
            </Table>
            <Empty v-else class="mb-4" description="尚未点亮" />

            <h3 class="mb-2 font-semibold">最近事件（{{ events.length }}）</h3>
            <Table
              v-if="events.length"
              :data-source="events"
              :pagination="false"
              row-key="id"
              size="small"
            >
              <TableColumn title="事件" data-index="name" :width="110" />
              <TableColumn title="时间" :width="170">
                <template #default="{ record }">{{ t2str(record.t) }}</template>
              </TableColumn>
              <TableColumn title="props">
                <template #default="{ record }">
                  <span class="truncate text-xs opacity-70">{{ record.props }}</span>
                </template>
              </TableColumn>
            </Table>
            <Empty v-else description="无事件" />
          </template>
        </Spin>
      </Drawer>
    </div>
  </Page>
</template>
