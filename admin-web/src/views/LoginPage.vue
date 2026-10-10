<template>
  <div style=" display: flex; align-items: center; justify-content: center;min-height: 100vh; padding: 16px">
    <n-card title="后台登录" style=" width: 100%;max-width: 420px">
      <n-form :model="form" :rules="rules" ref="formRef">
        <n-form-item label="用户名" path="username">
          <n-input v-model:value="form.username" placeholder="例如：demo" />
        </n-form-item>
        <n-form-item label="密码" path="password">
          <n-input v-model:value="form.password" type="password" placeholder="默认：admin123" />
        </n-form-item>
        <n-space justify="end">
          <n-button type="primary" :loading="loading" @click="onSubmit">登录</n-button>
        </n-space>
      </n-form>
    </n-card>
  </div>
</template>

<script setup lang="ts">
import type { FormInst, FormRules } from 'naive-ui'
import { useMessage } from 'naive-ui'
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { login } from '../api/admin'
import { useAuthStore } from '../stores/auth'

const router = useRouter()
const route = useRoute()
const message = useMessage()
const auth = useAuthStore()

const formRef = ref<FormInst | null>(null)
const loading = ref(false)
const form = reactive({ username: 'admin', password: 'admin123' })

const rules: FormRules = {
  username: [{ required: true, message: '请输入用户名', trigger: ['input', 'blur'] }],
  password: [{ required: true, message: '请输入密码', trigger: ['input', 'blur'] }],
}

async function onSubmit() {
  await formRef.value?.validate()
  loading.value = true
  try {
    const res = await login({ username: form.username, password: form.password })
    auth.setToken(res.token)
    message.success('登录成功')
    const redirect = (route.query.redirect as string) || '/'
    router.replace(redirect)
  } catch (e: any) {
    message.error(`登录失败：${e?.message || 'unknown error'}`)
  } finally {
    loading.value = false
  }
}
</script>
