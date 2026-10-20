<script lang="ts" setup>
import { formatDateTime } from '#/utils/format';
import { Descriptions } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenDrawer } from '@vben/common-ui';

interface DetailItem {
  label: string;
  value: string;
}

const items = ref<DetailItem[]>([]);

const [Drawer, drawerApi] = useVbenDrawer({
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      return;
    }
    const row = drawerApi.getData();
    items.value = [
      {
        label: 'id',
        value:
          row.id == null || row.id === ''
            ? '-'
            : String(row.id),
      },
      {
        label: '名称',
        value:
          row.name == null || row.name === ''
            ? '-'
            : String(row.name),
      },
      {
        label: '状态 1=启用 0=停用',
        value:
          row.status == null || row.status === ''
            ? '-'
            : String(row.status),
      },
      {
        label: '创建时间',
        value:
          row.created_at ? formatDateTime(row.created_at) : '-',
      },
      {
        label: '更新时间',
        value:
          row.updated_at ? formatDateTime(row.updated_at) : '-',
      },
    ];
  },
});
</script>

<template>
  <Drawer :footer="false" :title="'RoomUser 详情'">
    <Descriptions :column="1" bordered size="small">
      <Descriptions.Item v-for="f in items" :key="f.label" :label="f.label">
        {{ f.value }}
      </Descriptions.Item>
    </Descriptions>
  </Drawer>
</template>
