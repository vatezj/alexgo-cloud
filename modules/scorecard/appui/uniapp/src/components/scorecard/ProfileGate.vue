<script setup lang="ts">
/**
 * 统一资料弹框（头像 + 昵称）：开房硬门 / 进局软提醒 / 牌局内编辑 三处复用
 *
 * - mode="gate"（开房硬门）：「取消」+ 主按钮（默认「保存并开房」）；保存成功 emit saved 由父层续走开房，
 *   取消/X 关闭 emit cancelled（父层清挂起动作，不开房）
 * - mode="reminder"（进局软提醒）：无关闭图标、点遮罩不关，仅一个视觉弱化的「跳过」可关；
 *   保存或跳过均 emit（saved / skipped）→ 父层续走原进局动作
 * - mode="edit"（牌局内编辑）：「取消」+「保存」；保存成功关弹框，authStore.user 回写即刷新展示
 * - 保存链路全在本组件：有新选头像先 uploadAvatar 拿绝对地址 → updateMe → 用响应回写 authStore.user
 * - MP：open-type="chooseAvatar" 头像 + type="nickname" 昵称快捷填充；非 MP：chooseImage + wd-input
 * - 嵌套场景（AvatarPopup 内打开）传 root-portal 脱离父弹层层级
 */
import type { MeResult, UpdateMeReq } from '@/api/scorecard/types'
import { ApiError } from '@/api/core/handlers'
import { updateMe, uploadAvatar } from '@/api/scorecard/auth'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { isPlaceholderNickname } from '@/utils/scorecard/profile'

type GateMode = 'gate' | 'reminder' | 'edit'

interface Props {
  show: boolean
  mode?: GateMode
  /** 主按钮文案；缺省 gate=「保存并开房」、reminder/edit=「保存」 */
  primaryText?: string
  /** 嵌套在其他 wd-popup 内打开时置 true（脱离父层级，避免 fixed 定位被父 transform 影响） */
  rootPortal?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  mode: 'gate',
  primaryText: '',
  rootPortal: false,
})

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  /** 保存成功（authStore.user 已回写）：gate 父层续走开房 / reminder 父层续走进局 */
  (e: 'saved', res: MeResult): void
  /** reminder 仅「跳过」触发：父层续走原进局动作 */
  (e: 'skipped'): void
  /** gate/edit 取消或 X 关闭：父层清挂起动作 */
  (e: 'cancelled'): void
}>()

const authStore = useScorecardAuthStore()
const toast = useToast()

const nickname = ref('')
const avatarPath = ref('')
const saving = ref(false)

const isReminder = computed(() => props.mode === 'reminder')

const title = computed(() => (props.mode === 'edit' ? '修改资料' : '完善资料'))

const caption = computed(() => {
  if (props.mode === 'edit')
    return '修改头像与昵称，方便牌友辨认'
  if (isReminder.value)
    return '进局前设置头像与昵称，方便牌友辨认'
  return '开房前设置头像与昵称，方便牌友辨认'
})

const primaryLabel = computed(() => {
  if (props.primaryText)
    return props.primaryText
  return props.mode === 'gate' ? '保存并开房' : '保存'
})

/** 弹窗头像预览：新选临时路径优先，其次已有资料头像（只改昵称时不重传头像）。 */
const avatarPreview = computed(() => avatarPath.value || authStore.user?.avatar || '')

/** 打开时播种草稿：昵称非占位才预填，头像从零开始（避免误传旧头像）。 */
watch(() => props.show, (visible) => {
  if (!visible)
    return
  const nick = authStore.user?.nickname || ''
  nickname.value = nick && !isPlaceholderNickname(nick) ? nick : ''
  avatarPath.value = ''
})

/**
 * MP 头像选择回调：open-type="chooseAvatar" → e.detail.avatarUrl 为临时路径，保存时才上传持久化。
 * uni-app-types 的 ChooseavatarEvent = BaseEvent（无 detail 声明），运行时才有 → unknown 收窄。
 */
function onChooseAvatar(e: unknown) {
  const url = (e as { detail?: { avatarUrl?: string } } | null)?.detail?.avatarUrl
  if (url)
    avatarPath.value = url
}

/** 非 MP fallback：uni.chooseImage 选一张（取消静默）。 */
async function pickAvatarFallback() {
  try {
    const res = await uni.chooseImage({ count: 1, sizeType: ['compressed'] })
    const path = res.tempFilePaths?.[0]
    if (path)
      avatarPath.value = path
  }
  catch {
    // 取消/失败静默
  }
}

/** 业务信封错误本层提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/**
 * 保存：昵称必填且非占位；有新选头像先 uploadAvatar 拿绝对地址；
 * updateMe → 用响应回写 authStore.user → 关弹窗 → emit saved 由父层续走。
 */
async function onSave() {
  if (saving.value)
    return
  const nick = nickname.value.trim()
  if (!nick) {
    toast.warning('请输入昵称')
    return
  }
  if (isPlaceholderNickname(nick)) {
    toast.warning('请设置自己的昵称')
    return
  }
  if (!avatarPreview.value) {
    toast.warning('请选择头像')
    return
  }
  saving.value = true
  try {
    const req: UpdateMeReq = { nickname: nick }
    if (avatarPath.value)
      req.avatar = await uploadAvatar(avatarPath.value)
    const res = await updateMe(req)
    authStore.user = res
    emit('update:show', false)
    emit('saved', res)
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    saving.value = false
  }
}

function onSkip() {
  emit('update:show', false)
  emit('skipped')
}

function onCancel() {
  emit('update:show', false)
  emit('cancelled')
}

/** wd-popup 自动 v-model 回写：reminder 禁遮罩/无 X 不会走到；gate/edit 关闭即取消。 */
function onPopupUpdate(value: boolean) {
  if (!value)
    onCancel()
}
</script>

<template>
  <wd-popup
    :model-value="props.show"
    position="center"
    :closable="!isReminder"
    :close-on-click-modal="!isReminder"
    :root-portal="props.rootPortal"
    custom-style="width: 320px; border-radius: 22px; background: #fff; border: 1.5px solid var(--brass-a50);"
    @update:model-value="onPopupUpdate"
  >
    <view class="gate-box">
      <text class="gate-title">
        {{ title }}
      </text>
      <text class="gate-cap">
        {{ caption }}
      </text>

      <!-- 头像：MP 官方 chooseAvatar 按钮（临时路径，保存时上传）；其他端 chooseImage -->
      <view class="gate-av-wrap">
        <!-- #ifdef MP-WEIXIN -->
        <button
          class="gate-av-btn"
          open-type="chooseAvatar"
          @chooseavatar="onChooseAvatar"
        >
          <image
            v-if="avatarPreview"
            class="gate-av-img"
            :src="avatarPreview"
            mode="aspectFill"
          />
          <text
            v-else
            class="gate-av-hint"
          >
            ＋ 头像
          </text>
        </button>
        <!-- #endif -->
        <!-- #ifndef MP-WEIXIN -->
        <view
          class="gate-av-btn"
          @click="pickAvatarFallback"
        >
          <image
            v-if="avatarPreview"
            class="gate-av-img"
            :src="avatarPreview"
            mode="aspectFill"
          />
          <text
            v-else
            class="gate-av-hint"
          >
            ＋ 头像
          </text>
        </view>
        <!-- #endif -->
      </view>

      <!-- 昵称：MP type=nickname 触发微信昵称快捷填充；其他端 wd-input -->
      <!-- #ifdef MP-WEIXIN -->
      <input
        v-model="nickname"
        class="gate-nick-input"
        type="nickname"
        :maxlength="24"
        placeholder="昵称"
        placeholder-class="gate-nick-ph"
      >
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <wd-input
        v-model="nickname"
        class="gate-field"
        :maxlength="24"
        clearable
        placeholder="昵称"
      />
      <!-- #endif -->

      <!-- gate/edit：取消 + 主按钮；reminder：主按钮 + 弱化「跳过」（唯一可关闭途径） -->
      <view
        v-if="!isReminder"
        class="gate-btns"
      >
        <button
          class="btn ghost"
          :disabled="saving"
          @click="onCancel"
        >
          取消
        </button>
        <button
          class="btn brass"
          :disabled="saving"
          @click="onSave"
        >
          {{ saving ? '保存中…' : primaryLabel }}
        </button>
      </view>
      <view
        v-else
        class="gate-btns-reminder"
      >
        <button
          class="btn brass"
          :disabled="saving"
          @click="onSave"
        >
          {{ saving ? '保存中…' : primaryLabel }}
        </button>
        <text
          class="gate-skip"
          @click="onSkip"
        >
          跳过
        </text>
      </view>
    </view>
  </wd-popup>
</template>

<style lang="scss" scoped>
.gate-box {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  padding: 22px 20px 20px;
}

.gate-title {
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  color: var(--on-ink);
  text-align: center;
  letter-spacing: 0.05em;
}

.gate-cap {
  margin-top: 6px;
  font-size: 12px;
  color: var(--dim);
  text-align: center;
}

.gate-av-wrap {
  display: flex;
  justify-content: center;
  margin: 16px 0 12px;
}

.gate-av-btn {
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 72px;
  height: 72px;
  padding: 0;
  overflow: hidden;
  line-height: 1;
  background: transparent;
  border: 2px dashed var(--on-ink-a28);
  border-radius: 50%;
}

/* mp button 默认伪元素边框清零（与 .fan-cta .fc::after 同款处理） */
.gate-av-btn::after {
  border: none;
}

.gate-av-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.gate-av-hint {
  font-size: 13px;
  color: var(--brass);
}

/* MP 原生 input（type=nickname）：对齐 wd-input 视觉 */
.gate-nick-input {
  box-sizing: border-box;
  height: 44px;
  padding: 0 14px;
  font-size: 14px;
  color: var(--on-ink);
  background: var(--on-ink-a04);
  border-radius: 10px;
}

.gate-nick-ph {
  color: var(--dim);
}

.gate-field {
  background: var(--on-ink-a04);
  border-radius: 10px;
}

.gate-btns {
  display: flex;
  gap: 10px;
  margin-top: 16px;
}

.gate-btns .btn {
  flex: 1;
  height: 44px;
  font-size: 14px;
}

.gate-btns .btn.brass[disabled] {
  color: var(--dim);
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  box-shadow: none;
  opacity: 1;
}

/* reminder：主按钮 + 弱化「跳过」（视觉弱化但保留可点热区） */
.gate-btns-reminder {
  display: flex;
  flex-direction: column;
  gap: 6px;
  align-items: center;
  margin-top: 16px;
}

.gate-btns-reminder .btn {
  width: 100%;
  height: 44px;
  font-size: 14px;
}

.gate-btns-reminder .btn.brass[disabled] {
  color: var(--dim);
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  box-shadow: none;
  opacity: 1;
}

/* 跳过：小字号 + 弱色 + 最小热区（min-height 保证可点，仅视觉弱化） */
.gate-skip {
  min-height: 28px;
  padding: 6px 20px;
  font-size: 10px;
  line-height: 16px;
  color: var(--on-ink-a28);
  cursor: pointer;
}
</style>
