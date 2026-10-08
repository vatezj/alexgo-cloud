<template>
  <n-space vertical :size="12">
    <n-page-header title="系统用户" subtitle="/api/admin/system/users" />
    <n-card>
      <n-space :size="12" align="center" style="margin-bottom: 12px">
        <n-input v-model:value="createForm.username" placeholder="username" style="max-width: 180px" />
        <n-input v-model:value="createForm.nickname" placeholder="nickname" style="max-width: 180px" />
        <n-input v-model:value="createForm.password" type="password" placeholder="password" style="max-width: 180px" />
        <n-button type="primary" :loading="creating" @click="onCreate">创建</n-button>
        <n-button @click="refresh" :loading="loading">刷新</n-button>
      </n-space>
      <n-data-table :columns="columns" :data="rows" :loading="loading" :row-key="rowKey" />
    </n-card>
  </n-space>
</template>

<script setup lang="ts">
import type { DataTableColumns } from 'naive-ui'
import { NButton, NSpace, useMessage } from 'naive-ui'
import { h, onMounted, reactive, ref } from 'vue'
import { createUser, listUsers, resetUserPassword, type User } from '../../api/admin'

const message = useMessage()
const loading = ref(false)
const creating = ref(false)
const rows = ref<User[]>([])

const createForm = reactive({ username: '', nickname: '', password: '' })

const columns: DataTableColumns<User> = [
  { title: 'ID', key: 'id' },
  { title: 'Username', key: 'username' },
  { title: 'Nickname', key: 'nickname' },
  { title: 'Status', key: 'status' },
  { title: 'CreatedAt', key: 'created_at' },
  { title: 'UpdatedAt', key: 'updated_at' },
  {
    title: 'Actions',
    key: 'actions',
    render(row) {
      return h(
        NSpace,
        {},
        {
          default: () => [
            h(
              NButton,
              {
                size: 'small',
                onClick: async () => {
                  const pwd = window.prompt('新密码')
                  if (!pwd) return
                  try {
                    await resetUserPassword(Number(row.id), pwd)
                    message.success('重置成功')
                  } catch (e: any) {
                    message.error(`重置失败：${e?.message || 'unknown error'}`)
                  }
                },
              },
              { default: () => '重置密码' }
            ),
          ],
        }
      )
    },
  },
]

function rowKey(row: User) {
  return row.id
}

async function refresh() {
  loading.value = true
  try {
    const res = await listUsers()
    rows.value = res.data || []
  } catch (e: any) {
    message.error(`加载失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}

async function onCreate() {
  if (!createForm.username || !createForm.password) {
    message.error('请输入 username/password')
    return
  }
  creating.value = true
  try {
    await createUser({
      username: createForm.username,
      nickname: createForm.nickname,
      password: createForm.password,
    })
    message.success('创建成功')
    createForm.username = ''
    createForm.nickname = ''
    createForm.password = ''
    await refresh()
  } catch (e: any) {
    message.error(`创建失败：${e?.message || 'unknown error'}`)
  } finally {
    creating.value = false
  }
}

onMounted(refresh)
</script>
