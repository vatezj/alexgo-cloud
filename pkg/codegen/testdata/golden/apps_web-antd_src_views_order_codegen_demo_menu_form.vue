<script lang="ts" setup>
import dayjs from 'dayjs';
import { message } from 'ant-design-vue';
import { ref } from 'vue';

import { useVbenModal } from '@vben/common-ui';

import {
  createCodegenDemoMenu,
  listCodegenDemoMenu,
  updateCodegenDemoMenu,
} from '#/api/order/codegen_demo_menu/api';
import { useVbenForm } from '#/adapter/form';

const emit = defineEmits<{ saved: [] }>();

/** null = 新建；非空 = 正在编辑的记录 id */
const editingId = ref<number | null>(null);

/** 父级树：打开弹窗时拉平铺 list 本地构树（spec §10.4，后端 List 不构树）。
 * 走函数形式 componentProps——form-field.vue 的 computed 内求值，
 * 依赖 parentTree.value 响应式，晚到的异步数据也能刷新下拉。 */
interface TreeNode {
  children?: TreeNode[];
  title: string;
  value: number;
}
const parentTree = ref<TreeNode[]>([{ children: [], title: '顶级', value: 0 }]);

async function loadParentTree() {
  try {
    const rows = await listCodegenDemoMenu();
    const nodes = new Map<number, TreeNode>();
    for (const r of rows) {
      nodes.set(Number(r.id), {
        children: [],
        title: String(r.name ?? r.id),
        value: Number(r.id),
      });
    }
    const roots: TreeNode[] = [];
    for (const r of rows) {
      const node = nodes.get(Number(r.id));
      if (!node) continue;
      const pid = Number(r.parent_id ?? 0);
      // 根（parent_id=0）与孤儿（父已被删）都挂顶层。
      if (pid === 0 || !nodes.has(pid)) {
        roots.push(node);
      } else {
        nodes.get(pid)?.children?.push(node);
      }
    }
    // 根=0 可选（spec §10.4）：虚拟顶级节点 value=0，真实节点全挂其下。
    parentTree.value = [{ children: roots, title: '顶级', value: 0 }];
  } catch {
    // 拦截器已 toast；树数据留空仅影响下拉可选项
  }
}

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
    void loadParentTree(); // 异步拉取，与下面回填互不阻塞
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
    // 树表根=0：TreeSelect 未选/选顶级都归一为 0（spec §10.1/10.4）。
    payload.parent_id = Number(payload.parent_id ?? 0);
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
      // 列名硬认（spec §10.4）：parent_id 一律 TreeSelect，html_type 不参与。
      // 不设 rules:'required'——根节点 value=0 是合法默认，避免 required 卡 0 值。
      component: 'TreeSelect',
      componentProps: () => ({
        allowClear: true,
        treeData: parentTree.value,
        treeDefaultExpandAll: true,
      }),
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
