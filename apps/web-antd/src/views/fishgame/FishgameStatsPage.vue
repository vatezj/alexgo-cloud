<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';

import { Page } from '@vben/common-ui';

import { Card, Col, Row, Spin, Statistic, Table, TableColumn, Tag } from 'ant-design-vue';

import { getFishgameStatsApi } from '#/api/fishgame';

const loading = ref(false);
const stats = ref<any>(null);

async function load() {
  loading.value = true;
  try {
    stats.value = await getFishgameStatsApi();
  } finally {
    loading.value = false;
  }
}

const funnelRows = computed(() => {
  const f = stats.value?.funnel ?? {};
  const r = stats.value?.funnel_ratio ?? {};
  const label: Record<string, string> = {
    catch: '命中',
    empty: '空钩',
    escape: '逃脱',
    broken: '断线',
  };
  return ['catch', 'empty', 'escape', 'broken'].map((k) => ({
    key: k,
    name: label[k] ?? k,
    count: f[k] ?? 0,
    ratio: ((r[k] ?? 0) * 100).toFixed(1),
  }));
});

const eventRows = computed(() =>
  Object.entries(stats.value?.event_counts ?? {}).map(([name, count]) => ({ name, count })),
);

const dailyRows = computed(() => stats.value?.daily ?? []);

onMounted(load);
</script>

<template>
  <Page auto-content-height>
    <Spin :spinning="loading">
      <div class="p-4">
        <Row :gutter="16" class="mb-4">
          <Col :span="6">
            <Card>
              <Statistic :value="stats?.player_total ?? 0" title="玩家总数" />
            </Card>
          </Col>
          <Col :span="6">
            <Card>
              <Statistic :value="stats?.player_today ?? 0" title="今日新增" />
            </Card>
          </Col>
          <Col :span="6">
            <Card>
              <Statistic :value="stats?.sign_count ?? 0" title="签到（近7日）" />
            </Card>
          </Col>
          <Col :span="6">
            <Card>
              <Statistic :value="stats?.ad_count ?? 0" title="广告（近7日）" />
            </Card>
          </Col>
        </Row>

        <Row :gutter="16">
          <Col :span="12">
            <Card class="mb-4" title="四结果漏斗（近 7 日）">
              <Table :data-source="funnelRows" :pagination="false" row-key="key" size="small">
                <TableColumn title="结果" data-index="name" />
                <TableColumn title="次数" data-index="count" :width="90" />
                <TableColumn title="占比" :width="90">
                  <template #default="{ record }">
                    <Tag>{{ record.ratio }}%</Tag>
                  </template>
                </TableColumn>
              </Table>
            </Card>
          </Col>
          <Col :span="12">
            <Card class="mb-4" title="事件计数（近 7 日）">
              <Table
                :data-source="eventRows"
                :pagination="false"
                row-key="name"
                size="small"
              >
                <TableColumn title="事件" data-index="name" />
                <TableColumn title="次数" data-index="count" :width="90" />
              </Table>
            </Card>
          </Col>
        </Row>

        <Card title="按日趋势（近 7 日）">
          <Table :data-source="dailyRows" :pagination="false" row-key="day" size="small">
            <TableColumn title="日期" data-index="day" :width="130" />
            <TableColumn title="事件" data-index="name" :width="140" />
            <TableColumn title="次数" data-index="count" :width="90" />
          </Table>
        </Card>
      </div>
    </Spin>
  </Page>
</template>
