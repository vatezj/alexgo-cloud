<script lang="ts" setup>
import dayjs from 'dayjs';
import { message } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  createFriendBlack,
  updateFriendBlack,
} from '#/api/scorecard/friend_black/api';
import { useVbenForm } from '#/adapter/form';

const emit = defineEmits<{ saved: [] }>();

/** null = 新建；非空 = 正在编辑的记录 id */
const editingId = ref<number | null>(null);

const [Modal, modalApi] = useVbenModal({
  onConfirm: async () => {
    try {
      await formApi.validateAndSubmitForm();
    } catch {
      // 校验失败或接口报错：拦截器已 toast，弹窗保持打开
    }
  },
  onOpenChange(isOpen: boolean) {
    if (!isOpen) {
      editingId.value = null;
      return;
    }
    // open() 之后先重置再回填（与 SystemTenantsPage 同款时序）
    void (async () => {
      await formApi.resetForm();
      const row = modalApi.getData();
      if (row.id != null) {
        editingId.value = Number(row.id);
        await formApi.setValues({
          name: row.name,
          status: row.status,
          created_at: row.created_at ? dayjs(row.created_at) : undefined,
          updated_at: row.updated_at ? dayjs(row.updated_at) : undefined,
        });
      }
    })();
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    const payload: Record<string, any> = { ...values };
    if (editingId.value != null) {
      await updateFriendBlack(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createFriendBlack(payload);
      message.success('创建成功');
    }
    modalApi.close();
    emit('saved');
  },
  schema: [
    {
      component: 'Input',
      rules: 'required',
      fieldName: 'name',
      label: '名称',
    },
    {
      component: 'InputNumber',
      rules: 'required',
      fieldName: 'status',
      label: '状态 1=启用 0=停用',
    },
    {
      component: 'DatePicker',
      componentProps: { showTime: true },
      rules: 'required',
      fieldName: 'created_at',
      label: '创建时间',
    },
    {
      component: 'DatePicker',
      componentProps: { showTime: true },
      rules: 'required',
      fieldName: 'updated_at',
      label: '更新时间',
    },
  ],
  showDefaultActions: false,
});
</script>

<template>
  <Modal>
    <Form />
  </Modal>
</template>
