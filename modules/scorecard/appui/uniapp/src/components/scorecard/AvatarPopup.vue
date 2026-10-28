<script setup lang="ts">
/**
 * 头像弹窗（设计 10 · 昵称 / 语音 5 档 / 主题入口 / 退出房间）
 *
 * - 主弹窗（居中）：头像 + 昵称 +「✎ 修改资料」「查看资料」；VoiceToggle（5 档循环）；
 *   主题入口（4 色卡横排 → useScorecardTheme().setTheme——spec §3.9.3 裁定）；
 *   「退出房间」leave 二次确认 → 回首页 +「退出登录」清 authStore → login +「完成」关闭
 * - 修改资料：ProfileGate（edit 模式，root-portal 叠层）——头像+昵称与开房/进局同一弹框；
 *   保存成功回写 authStore.user，本层 computed 即刷新（房间页快照由轮询 BriefProvider 补齐）
 * - 查看资料：只读弹窗（昵称 / ID / 注册时间）
 * - 在房判定：本弹窗仅从房间页打开（房间上下文即在房），故同时给「退出房间」与「退出登录」
 * - 业务错误本地 toastApiError（ApiError 非 401/403）；色值零硬编码（主题 token +
 *   全局工具类 .btn.brass/.btn.ghost）；色卡自带 theme-t{n} 类借各主题 token 渲染预览
 */
import type { ScorecardThemeId } from '@/store/scorecard/theme'
import { ApiError } from '@/api/core/handlers'
import { leave } from '@/api/scorecard/room'
import ProfileGate from '@/components/scorecard/ProfileGate.vue'
import VoiceToggle from '@/components/scorecard/VoiceToggle.vue'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { useScorecardRoomStore } from '@/store/scorecard/room'
import { useScorecardTheme } from '@/store/scorecard/theme'

interface Props {
  show: boolean
  roomId: number
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

const router = useRouter()
const toast = useToast()
const dialog = useDialog()
const authStore = useScorecardAuthStore()
const roomStore = useScorecardRoomStore()
const { themeId, setTheme } = useScorecardTheme()

const me = computed(() => authStore.user)
const nickname = computed(() => me.value?.nickname || '牌友')
const initial = computed(() => (nickname.value.slice(0, 1) || '牌').toUpperCase())
const createdText = computed(() => {
  const raw = me.value?.created_at
  if (!raw)
    return '—'
  const d = new Date(raw)
  if (Number.isNaN(d.getTime()))
    return raw
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
})

/** 主题 4 色卡（横排；名称对稿 themes.html ①–④）。 */
const themeOptions: { id: ScorecardThemeId, short: string }[] = [
  { id: 1, short: '墨金' },
  { id: 2, short: '深海' },
  { id: 3, short: '红黑' },
  { id: 4, short: '青瓷' },
]

/** 业务信封错误本层提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

function close() {
  emit('update:show', false)
}

/* ── 修改资料（ProfileGate edit 模式，保存链路在组件内） ── */

const showProfileEdit = ref(false)

/** 保存成功（authStore.user 已由 ProfileGate 回写）：toast 反馈，本层昵称/头像 computed 即刷新。 */
function onProfileSaved() {
  toast.success('资料已更新')
}

/* ── 查看资料（只读） ── */

const showProfile = ref(false)

/* ── 退出房间 / 退出登录 ── */

const acting = ref(false)

/** 退出房间：二次确认 → leave → 清快照 → 回首页（房主退房后端自动结算）。 */
async function onLeaveRoom() {
  if (acting.value || !props.roomId)
    return
  acting.value = true
  try {
    try {
      await dialog.confirm({
        title: '退房确认',
        msg: '确认退出本房间？房主退房将自动结束本局。',
        confirmButtonText: '确认退房',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    await leave(props.roomId)
    roomStore.clearSnapshot()
    close()
    toast.success('已退房')
    router.replaceAll({ path: '/pages/index/index' })
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    acting.value = false
  }
}

/** 退出登录：二次确认 → 清 authStore → 登录页。 */
async function onLogout() {
  if (acting.value)
    return
  acting.value = true
  try {
    try {
      await dialog.confirm({
        title: '退出登录',
        msg: '确认退出当前账号？',
        confirmButtonText: '退出登录',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    authStore.logout()
    close()
    router.replaceAll({ path: '/pages/login/index' })
  }
  finally {
    acting.value = false
  }
}
</script>

<template>
  <!-- 主弹窗（设计 10 居中卡） -->
  <wd-popup
    :model-value="props.show"
    position="center"
    custom-style="width: 320px; border-radius: 22px; background: var(--ink-700); border: 1.5px solid var(--brass-a50);"
    @update:model-value="close"
  >
    <view class="avatar-popup">
      <!-- 头像 + 昵称 + 快捷入口 -->
      <view class="ap-head">
        <view class="ap-av">
          <image
            v-if="me?.avatar"
            class="ap-img"
            :src="me.avatar"
            mode="aspectFill"
          />
          <text
            v-else
            class="ap-init"
          >
            {{ initial }}
          </text>
        </view>
        <text class="ap-name">
          {{ nickname }}
        </text>
        <view class="ap-links">
          <text
            class="ap-link"
            @click="showProfileEdit = true"
          >
            ✎ 修改资料
          </text>
          <text
            class="ap-link on"
            @click="showProfile = true"
          >
            查看资料
          </text>
        </view>
      </view>

      <!-- 语音 5 档（设计 10 · 手册 §3.8） -->
      <view class="ap-sec">
        <VoiceToggle />
      </view>

      <!-- 主题入口：4 色卡横排（spec §3.9.3 裁定） -->
      <view class="ap-sec">
        <text class="ap-sec-label">
          主题
        </text>
        <view class="th-row">
          <view
            v-for="t in themeOptions"
            :key="t.id"
            class="th-card"
            :class="[`theme-t${t.id}`, { on: themeId === t.id }]"
            @click="setTheme(t.id)"
          >
            <view class="th-swatch" />
            <text class="th-name">
              {{ t.short }}
            </text>
          </view>
        </view>
      </view>

      <!-- 操作：退出房间 / 完成 + 退出登录 -->
      <view class="ap-btns">
        <button
          class="btn ghost leave-btn"
          :disabled="acting"
          @click="onLeaveRoom"
        >
          退出房间
        </button>
        <button
          class="btn brass"
          @click="close"
        >
          完成
        </button>
      </view>
      <text
        class="ap-logout"
        @click="onLogout"
      >
        退出登录
      </text>
      <text class="ap-cap">
        只能自己退房，没有踢人 · 数据为零可直接退
      </text>
    </view>
  </wd-popup>

  <!-- 修改资料弹窗（ProfileGate edit：与开房/进局同一弹框，root-portal 脱离主弹层层级） -->
  <ProfileGate
    v-model:show="showProfileEdit"
    mode="edit"
    :root-portal="true"
    @saved="onProfileSaved"
  />

  <!-- 查看资料弹窗（只读） -->
  <wd-popup
    v-model="showProfile"
    position="center"
    root-portal
    custom-style="width: 300px; border-radius: 18px; background: var(--ink-700); border: 1.5px solid var(--brass-a50);"
  >
    <view class="mini-popup">
      <text class="mp-title">
        查看资料
      </text>
      <view class="pf-row">
        <text class="pf-k">
          昵称
        </text>
        <text class="pf-v">
          {{ nickname }}
        </text>
      </view>
      <view class="pf-row">
        <text class="pf-k">
          ID
        </text>
        <text class="pf-v num">
          {{ me?.user_id ?? '—' }}
        </text>
      </view>
      <view class="pf-row">
        <text class="pf-k">
          注册时间
        </text>
        <text class="pf-v">
          {{ createdText }}
        </text>
      </view>
      <button
        class="btn brass"
        @click="showProfile = false"
      >
        完成
      </button>
    </view>
  </wd-popup>
</template>

<style lang="scss" scoped>
.avatar-popup {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 20px 20px 18px;
}

/* ── 头像 + 昵称 ── */
.ap-head {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: center;
}

.ap-av {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 64px;
  height: 64px;
  overflow: hidden;
  background: var(--ink-600);
  border: 3px solid var(--brass);
  border-radius: 50%;
}

.ap-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.ap-init {
  font-family: 'Noto Serif SC', serif;
  font-size: 22px;
  color: var(--brass);
}

.ap-name {
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  color: var(--on-ink);
  letter-spacing: 0.05em;
}

.ap-links {
  display: flex;
  gap: 16px;
  margin-top: 4px;
  font-size: 13px;
}

.ap-link {
  color: var(--dim);
  cursor: pointer;
}

.ap-link.on {
  color: var(--brass);
}

/* ── 分节 ── */
.ap-sec {
  padding-top: 12px;
  margin-top: 16px;
  border-top: 1px solid var(--on-ink-a25);
}

.ap-sec-label {
  display: block;
  margin-bottom: 8px;
  font-size: 14px;
  color: var(--on-ink);
}

/* ── 主题 4 色卡横排 ── */
.th-row {
  display: flex;
  gap: 8px;
}

.th-card {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 5px;
  padding: 6px;
  cursor: pointer;
  border: 1.5px solid var(--on-ink-a28);
  border-radius: 10px;
}

.th-card.on {
  border-color: var(--brass);
}

.th-swatch {
  position: relative;
  height: 30px;
  background: var(--felt);
  border: 1px solid var(--brass-a50);
  border-radius: 7px;
}

/* 黄铜条 + 赢色点：用该主题自身 token 画预览（零硬编码色） */
.th-swatch::before {
  position: absolute;
  top: 6px;
  right: 6px;
  left: 6px;
  height: 7px;
  content: '';
  background: var(--brass);
  border-radius: 4px;
}

.th-swatch::after {
  position: absolute;
  right: 6px;
  bottom: 5px;
  width: 8px;
  height: 8px;
  content: '';
  background: var(--win-dark);
  border-radius: 50%;
}

.th-name {
  font-size: 10.5px;
  color: var(--dim);
  text-align: center;
}

.th-card.on .th-name {
  color: var(--brass);
}

/* ── 操作按钮 ── */
.ap-btns {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.ap-btns .btn {
  flex: 1;
  height: 48px;
  font-size: 15px;
}

.leave-btn {
  color: var(--vermillion-hi);
  border-color: var(--comply-border);
}

.ap-logout {
  display: block;
  margin-top: 10px;
  font-size: 12.5px;
  color: var(--dim);
  text-align: center;
  cursor: pointer;
}

.ap-cap {
  display: block;
  margin-top: 9px;
  font-size: 11px;
  color: var(--dim);
  text-align: center;
}

/* ── 内层小弹窗（查看资料） ── */
.mini-popup {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 20px 18px 18px;
}

.mp-title {
  margin-bottom: 12px;
  font-family: 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 700;
  color: var(--on-ink);
  text-align: center;
  letter-spacing: 0.05em;
}

/* ── 资料行 ── */
.pf-row {
  display: flex;
  gap: 12px;
  align-items: center;
  justify-content: space-between;
  padding: 9px 2px;
  border-bottom: 1px solid var(--on-ink-a04);
}

.pf-k {
  flex: none;
  font-size: 12.5px;
  color: var(--dim);
}

.pf-v {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  font-size: 13.5px;
  color: var(--on-ink);
  text-align: right;
  white-space: nowrap;
}

.mini-popup .btn.brass {
  height: 44px;
  margin-top: 14px;
  font-size: 14px;
}
</style>
