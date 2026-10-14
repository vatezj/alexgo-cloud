<script lang="ts" setup>
import dayjs from 'dayjs';
import { message } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  createCodegenDemoMenu,
  updateCodegenDemoMenu,
} from '#/api/order/codegen_demo_menu/api';
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
          parent_id: row.parent_id,
          name: row.name,
          remark: row.remark,
          created_at: row.created_at ? dayjs(row.created_at) : undefined,
        });
      }
    })();
  },
});

const [Form, formApi] = useVbenForm({
  handleSubmit: async (values) => {
    const payload: Record<string, any> = { ...values };
    if (editingId.value != null) {
      await updateCodegenDemoMenu(editingId.value, payload);
      message.success('保存成功');
    } else {
      await createCodegenDemoMenu(payload);
      message.success('创建成功');
    }
    modalApi.close();
    emit('saved');
  },
  schema: [
    {
      component: 'InputNumber',
      rules: 'required',
      fieldName: 'parent_id',
      label: '父级ID',
    },
    {
      component: 'Input',
      rules: 'required',
      fieldName: 'name',
      label: '名称',
    },
    {
      component: 'Input',
      fieldName: 'remark',
      label: '备注',
    },
    {
      component: 'DatePicker',
      componentProps: { showTime: true },
      rules: 'required',
      fieldName: 'created_at',
      label: '创建时间',
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
