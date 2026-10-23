<script setup lang="ts">
/**
 * 登录页（墨金门厅）：微信一键登录（mp）+ 手机号密码表单（H5 默认展开，mp 折叠）
 *
 * - 业务信封错误（含 5xxxx）不全局 toast：catch ApiError 后本页 useToast 提示 err.msg
 * - 401/403（handlers 清态跳登录）与网络/超时错误已由 handlers 全局提示，本页不重复 toast
 * - 登录成功统一走 afterLogin：setLogin → fetchMe → pendingScene 有房号则 joinRoom 进房，否则回首页
 */
import type { LoginResult } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { loginByMobile, wxLogin } from '@/api/scorecard/auth'
import { joinRoom } from '@/api/scorecard/room'
import { useSceneRoom } from '@/composables/scorecard/useSceneRoom'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardAuthStore } from '@/store/scorecard/auth'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const authStore = useScorecardAuthStore()
const router = useRouter()
const toast = useToast()
const { consumeScene } = useSceneRoom()
const { pageStyle } = useSystemBar()

const logging = ref(false)
const mobile = ref('')
const password = ref('')
// H5 无 wx.login：默认展开密码表单；mp 默认折叠（微信一键登录优先）
const showPwdForm = ref(false)
// #ifdef H5
showPwdForm.value = true
// #endif

/** 业务信封错误本页提示；401/403 与其他已全局提示的错误不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '登录失败' })
}

/** 登录成功统一收口：存 token（persist 落盘）→ 拉资料 → scene 房号只消费一次（有则进房，否则回首页）。 */
async function afterLogin(res: LoginResult) {
  authStore.setLogin(res)
  try {
    await authStore.fetchMe()
  }
  catch {
    // 资料缓存失败不阻断登录（token 已落 store 并持久化）
  }
  const roomCode = consumeScene()
  if (roomCode) {
    try {
      const snapshot = await joinRoom({ room_code: roomCode })
      authStore.lastRoomId = snapshot.id
      router.push({ path: '/pages/room/index', query: { roomId: String(snapshot.id) } })
      return
    }
    catch (err) {
      toastApiError(err)
    }
  }
  router.replaceAll({ path: '/pages/index/index' })
}

/** mp：微信一键登录。uni.login 失败本地提示可重试；接口错误走 toastApiError。 */
async function onWxLogin() {
  if (logging.value)
    return
  logging.value = true
  try {
    // #ifdef MP-WEIXIN
    let code = ''
    try {
      const loginRes = await uni.login()
      code = loginRes.code || ''
    }
    catch {
      toast.error('微信登录失败，请重试')
      return
    }
    if (!code) {
      toast.error('微信登录失败，请重试')
      return
    }
    await afterLogin(await wxLogin(code))
    // #endif
    // #ifndef MP-WEIXIN
    toast.error('请在微信小程序中使用微信登录')
    // #endif
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    logging.value = false
  }
}

async function onPasswordLogin() {
  if (logging.value)
    return
  if (!/^1\d{10}$/.test(mobile.value)) {
    toast.warning('请输入正确的手机号')
    return
  }
  if (!password.value) {
    toast.warning('请输入密码')
    return
  }
  logging.value = true
  try {
    const res = await loginByMobile({ mobile: mobile.value, password: password.value })
    await afterLogin(res)
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    logging.value = false
  }
}
</script>

<template>
  <view
    class="login-page"
    :style="pageStyle"
  >
    <view class="brand">
      <text class="brand-title">
        打牌记账
      </text>
      <text class="brand-sub">
        开一桌 · 账目分明
      </text>
    </view>

    <view class="pcard login-card">
      <text class="idx tl">
        A<span class="s">♠</span>
      </text>
      <text class="idx br red">
        A<span class="s">♥</span>
      </text>
      <text class="card-title">
        入座
      </text>

      <!-- #ifdef MP-WEIXIN -->
      <button
        class="btn brass"
        :disabled="logging"
        @click="onWxLogin"
      >
        {{ logging ? '登录中…' : '微信一键登录' }}
      </button>
      <button
        class="form-toggle"
        @click="showPwdForm = !showPwdForm"
      >
        {{ showPwdForm ? '收起手机号登录' : '手机号登录' }}
      </button>
      <!-- #endif -->

      <view
        v-if="showPwdForm"
        class="form"
      >
        <wd-input
          v-model="mobile"
          class="field"
          type="tel"
          :maxlength="11"
          clearable
          placeholder="手机号"
        />
        <wd-input
          v-model="password"
          class="field"
          :maxlength="64"
          show-password
          placeholder="密码"
        />
        <button
          class="btn brass"
          :disabled="logging"
          @click="onPasswordLogin"
        >
          {{ logging ? '登录中…' : '登录' }}
        </button>
      </view>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.login-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 48px) 28px 32px;
  background: var(--ink-900);
}

.brand {
  margin-bottom: 36px;
  text-align: center;
}

.brand-title {
  display: block;
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  font-weight: 700;
  letter-spacing: 0.12em;
  color: var(--brass);
}

.brand-sub {
  display: block;
  margin-top: 8px;
  font-size: 12px;
  letter-spacing: 0.3em;
  color: var(--dim);
}

.login-card {
  padding: 34px 22px 26px;
}

.card-title {
  display: block;
  margin-bottom: 22px;
  font-family: 'Noto Serif SC', serif;
  font-size: 18px;
  font-weight: 700;
  text-align: center;
  color: var(--jet);
}

.form {
  margin-top: 18px;
}

.field {
  // 骨白牌面内的输入槽：全部走主题 token，无硬编码色
  --wot-input-bg: var(--tile-shade);
  --wot-input-inner-color: var(--jet);
  --wot-input-inner-placeholder-color: var(--tile-emboss);

  margin-bottom: 12px;
  border-radius: 10px;
}

.form-toggle {
  display: block;
  width: 100%;
  margin-top: 14px;
  padding: 6px 0;
  border: none;
  background: transparent;
  color: var(--jet);
  font-size: 13px;
  line-height: 1.4;
  text-align: center;
}

.form-toggle::after {
  border: none;
}
</style>
