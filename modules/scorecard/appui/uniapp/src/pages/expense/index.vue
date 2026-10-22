<script setup lang="ts">
/**
 * 支出页 · 筹码加减档（设计 04；Task 8 自占位页整体重写）
 *
 * - 分数档：ExpenseLane（±步进 10 + 预设 10/50/100/200）；score>0 才可提交
 * - 记分对象：query toUserId 预选（须为房内非自己成员），否则成员横选（排除自己）
 * - 茶饭档：tea_auto_enabled=true → 「按 X% 自动抽取」说明；false → 手动 tea 档 +
 *   扣茶来源横选（tea_from_user_id，默认支出方=我；0 由后端缺省=支出方）
 * - 幂等提交：reqId 页面进入生成一次存 ref，失败 toast 后重试复用同键
 *   （Review Focus #1：断网重按不双记，后端同 client_request_id 回放同 log_id）
 * - 成功：applySnapshot（响应内嵌快照，version 单调门）+ polling.refresh() 重置 +
 *   toast「已记分」+ 语音播报（speak ledger 类，永不 throw）+ navigateBack
 * - 业务错误本页 toast（ApiError 非 401/403）；401 由 handlers 全局清态跳登录
 * - 色值零硬编码：全走主题 token 与全局工具类（.comply/.num/.btn.brass）
 */
import { ApiError } from '@/api/core/handlers'
import { expense, snapshot as fetchSnapshot } from '@/api/scorecard/room'
import ComplianceBanner from '@/components/scorecard/ComplianceBanner.vue'
import ExpenseLane from '@/components/scorecard/ExpenseLane.vue'
import { useRoomPolling } from '@/composables/scorecard/useRoomPolling'
import { useVoice } from '@/composables/scorecard/useVoice'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { useScorecardRoomStore } from '@/store/scorecard/room'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const authStore = useScorecardAuthStore()
const roomStore = useScorecardRoomStore()
const { speak } = useVoice()

/**
 * query.roomId → number（非法/缺失返回0）。
 * 读 router.currentRoute 而非 useRoute() 副本：后者的 reactive 拷贝依赖 watcher flush，
 * 应用内跳转的 onShow 时机仍是上一路由的 query；currentRoute 由全局 mixin 在 onLoad
 * 同步写入（先于组件 onShow），任何读取时机都是本页 query（R1 Important）。
 */
const routeRoomId = computed(() => {
  const raw = router.currentRoute.value.query.roomId
  const str = Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? '')
  const n = Number(str)
  return Number.isInteger(n) && n > 0 ? n : 0
})

/** query.toUserId → 预选记分对象（非法/缺失返回0，不预选）；同 routeRoomId 读 currentRoute。 */
const routeToUserId = computed(() => {
  const raw = router.currentRoute.value.query.toUserId
  const str = Array.isArray(raw) ? (raw[0] ?? '') : (raw ?? '')
  const n = Number(str)
  return Number.isInteger(n) && n > 0 ? n : 0
})

/**
 * 本次房间 id：query 优先，兜底 store / lastRoomId（回访直达，与 room/settle 三段兜底链一致）。
 * 必须惰性求值（computed）：@wot-ui/router 的 currentRoute 在 setup 之后（onLoad/onShow）
 * 才同步本页 query，setup 时同步读到的是上一路由的值（R1 Important）。
 */
const roomId = computed(() => routeRoomId.value || roomStore.roomId || authStore.lastRoomId)

const polling = useRoomPolling(roomId)

const snapshot = computed(() =>
  roomStore.snapshot && roomStore.snapshot.id === roomId.value ? roomStore.snapshot : null,
)
const myUserId = computed(() => authStore.user?.user_id ?? 0)
const myNickname = computed(() => authStore.user?.nickname || '牌友')
const members = computed(() => snapshot.value?.members ?? [])
/** 记分对象候选：排除自己（后端拒 to_user_id==本人）。 */
const payees = computed(() => members.value.filter(m => m.user_id !== myUserId.value))
const teaAuto = computed(() => snapshot.value?.tea_auto_enabled ?? false)
const teaPercentText = computed(() => `${snapshot.value?.tea_percent ?? 0}%`)

const score = ref(0)
const tea = ref(0)
const toUserId = ref(0)
/** 手动茶扣来源：0 = 缺省=支出方（我）；选中后写具体成员 id。 */
const teaFromUserId = ref(0)
const submitting = ref(false)

/** 幂等键：页面进入生成一次，失败重试复用（Review Focus #1 防双记）。 */
const reqId = ref(genUuid())

/** UUID v4（Math.random 实现，兼容小程序端无 crypto.randomUUID）。 */
function genUuid(): string {
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0
    const v = c === 'x' ? r : (r & 0x3) | 0x8
    return v.toString(16)
  })
}

const roomCodeText = computed(() => {
  const code = snapshot.value?.room_code
  return code ? `${code.toUpperCase()} 房间` : '房间'
})

const canSubmit = computed(() => score.value > 0 && toUserId.value > 0 && !submitting.value)

/** 业务信封错误本页提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/** 回房间：有历史栈则 navigateBack，直达落地（H5 刷新）则 replaceAll 回房。 */
function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/room/index', query: { roomId: String(roomId.value) } })
}

/** query 预选：成员加载后校验「在房且非自己」才生效（过期/非法 query 静默忽略）。 */
watch(
  [payees, myUserId],
  () => {
    if (toUserId.value)
      return
    const t = routeToUserId.value
    if (t && payees.value.some(m => m.user_id === t))
      toUserId.value = t
  },
  { immediate: true },
)

/** 预选复检：auth 未就绪（myUserId=0）时 query 可能预选到自己，me 就绪后清零重选。 */
watch(myUserId, (id) => {
  if (id > 0 && toUserId.value === id)
    toUserId.value = 0
}, { immediate: true })

/** 扣茶来源默认支出方（我）：me 资料就绪后回填。 */
watch(
  myUserId,
  (id) => {
    if (!teaFromUserId.value && id > 0)
      teaFromUserId.value = id
  },
  { immediate: true },
)

async function onSubmit() {
  if (submitting.value)
    return
  if (score.value <= 0) {
    toast.warning('请先选择支出分数')
    return
  }
  if (!toUserId.value) {
    toast.warning('请先选择记分对象')
    return
  }
  submitting.value = true
  try {
    const res = await expense(roomId.value, {
      score: score.value,
      tea: teaAuto.value ? 0 : tea.value,
      to_user_id: toUserId.value,
      tea_from_user_id: teaAuto.value ? 0 : teaFromUserId.value,
      client_request_id: reqId.value,
    })
    // 响应内嵌最新快照：version 单调门写回（防旧覆盖新）
    roomStore.applySnapshot(res.snapshot)
    void polling.refresh().catch(() => {})
    toast.success('已记分')
    // 语音播报（ledger 类）：纯记分词汇拼接；speak 永不 throw，失败不影响回房
    speak(`${myNickname.value}支出${score.value}分`, 'ledger')
    goBack()
  }
  catch (err) {
    // reqId 保留：失败重试复用同键，后端幂等回放同 log_id，断网重按不双记
    toastApiError(err)
  }
  finally {
    submitting.value = false
  }
}

/* ── 生命周期 ── */

onShow(() => {
  if (!roomId.value) {
    toast.error('房间信息缺失')
    router.replaceAll({ path: '/pages/index/index' })
    return
  }
  if (!authStore.user)
    void authStore.fetchMe().catch(() => {})
  // 快照兜底（H5 直达/刷新时 store 可能无本房快照）；applySnapshot 走 version 单调门
  if (roomStore.snapshot?.id !== roomId.value) {
    fetchSnapshot(roomId.value)
      .then(snap => roomStore.applySnapshot(snap))
      .catch(toastApiError)
  }
})
</script>

<template>
  <view class="expense-page">
    <!-- 自绘 nav -->
    <view class="nav">
      <view
        class="side"
        @click="goBack"
      >
        <text class="navicon">
          ‹
        </text>
      </view>
      <view class="ntitle">
        支出
        <text class="nsub">
          {{ roomCodeText }} · {{ myNickname }}
        </text>
      </view>
      <view class="side right">
        <view class="capsule">
          <text class="cap-dots">
            •••
          </text>
          <view class="sep" />
          <view class="cap-c2">
            <view class="ring" />
          </view>
        </view>
      </view>
    </view>

    <!-- 合规横幅（锁定语，随全局本会话记忆） -->
    <ComplianceBanner />

    <!-- 分数档 -->
    <ExpenseLane
      v-model:value="score"
      lane="score"
      title="支出分数"
      tag="SCORE"
      :presets="[10, 50, 100, 200]"
      :step="10"
      :who-avatar="authStore.user?.avatar || ''"
      :who-name="myNickname"
    />

    <!-- 记分对象横选（query 预选后仍可改） -->
    <view class="payees">
      <text class="sec-label">
        记给谁
      </text>
      <scroll-view
        class="pay-scroll"
        scroll-x
        :show-scrollbar="false"
      >
        <view class="pay-inner">
          <text
            v-if="!payees.length"
            class="pay-empty"
          >
            等待其他牌友入座…
          </text>
          <view
            v-for="m in payees"
            :key="m.user_id"
            class="pay-chip"
            :class="{ on: toUserId === m.user_id }"
            @click="toUserId = m.user_id"
          >
            <view class="pc-av">
              <image
                v-if="m.avatar"
                class="pc-img"
                :src="m.avatar"
                mode="aspectFill"
              />
              <text
                v-else
                class="pc-txt"
              >
                {{ (m.nickname.slice(0, 1) || '牌').toUpperCase() }}
              </text>
            </view>
            <text class="pc-nm">
              {{ m.nickname || '牌友' }}
            </text>
          </view>
        </view>
      </scroll-view>
    </view>

    <!-- 茶饭档：自动抽取说明 / 手动加减档 -->
    <view
      v-if="teaAuto"
      class="auto-note"
    >
      <text class="an-t">
        ☕ 按 {{ teaPercentText }} 自动抽取
      </text>
      <text class="an-s">
        记分时自动抽茶入池，无需手动
      </text>
    </view>
    <template v-else>
      <ExpenseLane
        v-model:value="tea"
        lane="pot"
        title="支出茶/饭"
        tag="茶饭锅"
        :presets="[5, 10, 20]"
        :step="5"
      />
      <view class="tea-from">
        <text class="sec-label">
          扣茶来源
        </text>
        <scroll-view
          class="tf-scroll"
          scroll-x
          :show-scrollbar="false"
        >
          <view class="tf-inner">
            <view
              v-for="m in members"
              :key="m.user_id"
              class="tf-chip"
              :class="{ on: teaFromUserId === m.user_id }"
              @click="teaFromUserId = m.user_id"
            >
              <text>{{ m.user_id === myUserId ? '我' : (m.nickname || '牌友') }}</text>
            </view>
          </view>
        </scroll-view>
      </view>
    </template>

    <view class="grow" />

    <!-- 提交区 -->
    <view class="foot">
      <button
        class="btn brass submit-btn"
        :disabled="!canSubmit"
        @click="onSubmit"
      >
        支出 {{ score || 0 }} 分
      </button>
      <text class="cap">
        只记分数，不涉及金钱
      </text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.expense-page {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top, 0px) + 6px) 0 0;
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav（同房间页） ── */
.nav {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px 8px;
}

.nav .side {
  display: flex;
  min-width: 60px;
  align-items: center;
}

.nav .side.right {
  justify-content: flex-end;
}

.navicon {
  margin-right: 8px;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  line-height: 1;
}

.ntitle {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-align: center;
}

.nsub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

/* 胶囊占位（装饰，对齐源稿右上 capsule） */
.capsule {
  display: flex;
  width: 88px;
  height: 32px;
  align-items: center;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
}

.cap-dots {
  flex: 1;
  color: var(--on-ink);
  font-size: 14px;
  letter-spacing: 1.5px;
  line-height: 1;
  text-align: center;
}

.capsule .sep {
  width: 1px;
  height: 17px;
  background: var(--on-ink-a28);
}

.cap-c2 {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
}

.capsule .ring {
  position: relative;
  width: 13px;
  height: 13px;
  border: 2px solid var(--on-ink);
  border-radius: 50%;
}

.capsule .ring::after {
  position: absolute;
  inset: 3px;
  border-radius: 50%;
  background: var(--on-ink);
  content: '';
}

/* ── 分区标签 ── */
.sec-label {
  display: block;
  margin: 14px 20px 8px;
  color: var(--dim);
  font-size: 11px;
  letter-spacing: 0.12em;
}

/* ── 记分对象横选 ── */
.payees {
  margin-top: 2px;
}

.pay-scroll {
  width: 100%;
  white-space: nowrap;
}

.pay-inner {
  display: inline-flex;
  gap: 8px;
  padding: 0 20px;
}

.pay-chip {
  display: inline-flex;
  flex: none;
  align-items: center;
  gap: 6px;
  padding: 5px 12px 5px 5px;
  border: 1.5px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--ink-700);
  cursor: pointer;
}

.pay-chip.on {
  border-color: var(--brass);
  background: var(--on-ink-a04);
}

.pc-av {
  display: flex;
  width: 30px;
  height: 30px;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: var(--ink-600);
  color: var(--brass);
  font-size: 12px;
  font-weight: 500;
  overflow: hidden;
}

.pay-chip.on .pc-av {
  border: 1.5px solid var(--brass);
}

.pc-img {
  width: 100%;
  height: 100%;
  border-radius: 50%;
}

.pc-nm {
  max-width: 84px;
  overflow: hidden;
  color: var(--dim);
  font-size: 12.5px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pay-chip.on .pc-nm {
  color: var(--brass);
}

.pay-empty {
  padding: 6px 2px;
  color: var(--dim);
  font-size: 12px;
  white-space: normal;
}

/* ── 自动抽取说明 ── */
.auto-note {
  display: flex;
  margin: 12px 14px 0;
  padding: 16px 14px;
  flex-direction: column;
  gap: 6px;
  border: 1px solid var(--on-ink-a25);
  border-radius: 20px;
  background: var(--ink-700);
  position: relative;
}

.auto-note::before {
  position: absolute;
  top: 0;
  right: 18px;
  left: 18px;
  height: 2px;
  background: linear-gradient(90deg, transparent, var(--jade-hi), transparent);
  content: '';
  opacity: 0.7;
}

.an-t {
  color: var(--jade-hi);
  font-size: 15px;
  font-weight: 500;
}

.an-s {
  color: var(--dim);
  font-size: 12px;
}

/* ── 扣茶来源横选 ── */
.tea-from {
  margin-top: 2px;
}

.tf-scroll {
  width: 100%;
  white-space: nowrap;
}

.tf-inner {
  display: inline-flex;
  gap: 8px;
  padding: 0 20px;
}

.tf-chip {
  display: inline-flex;
  flex: none;
  align-items: center;
  padding: 7px 14px;
  border: 1px dashed var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
  color: var(--dim);
  cursor: pointer;
  font-size: 12.5px;
}

.tf-chip.on {
  border-color: var(--jade-hi);
  border-style: solid;
  color: var(--jade-hi);
}

.grow {
  flex: 1;
}

/* ── 提交区 ── */
.foot {
  flex: none;
  padding: 10px 20px calc(env(safe-area-inset-bottom, 0px) + 16px);
}

.submit-btn[disabled] {
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  box-shadow: none;
  color: var(--dim);
  opacity: 1;
}

.foot .cap {
  display: block;
  margin-top: 10px;
  color: var(--dim);
  font-size: 11px;
  text-align: center;
}
</style>
