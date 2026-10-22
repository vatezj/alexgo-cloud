<script setup lang="ts">
/**
 * 好友排名页 · 设计 08（Task 14 整体重写占位页）
 *
 * - 顶部汇总：好友数 + 「加入排名/关闭后不参与好友榜单」开关（我方裁定：
 *   无全局参与标志，初始按「全部我方 friend 行 rank_enabled 均为 true」推导，
 *   仅在用户改动时对全部 friend 行逐行 setRank，失败行 toast 继续下一行）
 * - 列表：listFriends（total_score 降序，单列表无分页）行=名次/头像/昵称/胜率/总分；
 *   自己若在榜标「· 我」；若列表不含自己，用 stat 补一行（昵称头像取 authStore.user）
 * - 左滑（FriendRow）：删除 dialog 二次确认 → removeFriend；拉黑 setBlack(true)；
 *   已拉黑行标记 +「解除」→ setBlack(false)
 * - 头部「+」→ 弹层输入对方 user_id → addFriend（30109/30108/40004 透出后端 msg）
 * - 业务错误本页 toast（ApiError 非 401/403）；401 由 handlers 全局清态跳登录
 * - 色值零硬编码：全走主题 token + ScoreText；Oswald 走全局 .num
 * - 文案红线：只用记分/排名词汇
 */
import type { FriendItem, StatResult } from '@/api/scorecard/types'
import { useQueue } from '@wot-ui/ui'
import { ApiError } from '@/api/core/handlers'
import { addFriend, listFriends, removeFriend, setBlack, setRank } from '@/api/scorecard/friend'
import { stat } from '@/api/scorecard/record'
import FriendRow from '@/components/scorecard/FriendRow.vue'
import { useScorecardAuthStore } from '@/store/scorecard/auth'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const dialog = useDialog()
const { closeOutside } = useQueue()
const authStore = useScorecardAuthStore()

const myUserId = computed(() => authStore.user?.user_id ?? 0)

/* ── 列表数据（单列表，无分页） ── */

const friends = ref<FriendItem[]>([])
const statResult = ref<StatResult | null>(null)
const loading = ref(false)
/** 首屏骨架（首拉未回时 true，控制空态文案不闪现）。 */
const bootstrapped = ref(false)

/** 排行展示行：API 行 + 必要时补的自己行，统一按 total_score 降序。 */
const rows = computed(() => {
  const list = [...friends.value]
  const selfInList = myUserId.value !== 0 && list.some(f => f.user_id === myUserId.value)
  if (!selfInList && myUserId.value !== 0 && statResult.value) {
    list.push({
      user_id: myUserId.value,
      nickname: authStore.user?.nickname || '我',
      avatar: authStore.user?.avatar || '',
      rank_enabled: true,
      total_score: statResult.value.total_score,
      win_rate: statResult.value.win_rate,
      is_blocked: false,
    })
  }
  return list.sort((a, b) => b.total_score - a.total_score)
})

const friendCount = computed(() => friends.value.length)

/* ── 加入排名汇总开关 ── */

/**
 * 无全局参与标志：初始按全部我方 friend 行 rank_enabled 均 true 推导（空列表视为开）。
 * 仅在用户改动时逐行 setRank 联动；刷新后按最新行状态重推。
 */
const rankOn = ref(true)
const rankBusy = ref(false)

function deriveRankOn() {
  if (!friends.value.length) {
    rankOn.value = true
    return
  }
  rankOn.value = friends.value.every(f => f.rank_enabled)
}

async function loadFriends() {
  if (loading.value)
    return
  loading.value = true
  try {
    friends.value = await listFriends()
    deriveRankOn()
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    loading.value = false
    bootstrapped.value = true
  }
}

async function loadStat() {
  try {
    statResult.value = await stat()
  }
  catch (err) {
    toastApiError(err)
  }
}

/** 刷新整页数据（增删/拉黑/开关后 / onShow）。 */
function refresh() {
  void loadFriends()
  void loadStat()
}

/** 开关切换：逐行 setRank 我方指向行；失败行 toast 后继续下一行。 */
async function onRankChange(e: { value: boolean }) {
  if (rankBusy.value)
    return
  const target = e.value
  // 只联动我方指向行（不含补出来的自己行）
  const targets = friends.value.filter(f => f.user_id !== myUserId.value)
  if (!targets.length) {
    rankOn.value = target
    return
  }
  rankBusy.value = true
  try {
    for (const f of targets) {
      try {
        await setRank(f.user_id, target)
      }
      catch (err) {
        toastApiError(err)
      }
    }
    toast.success(target ? '已加入排名' : '已关闭排名参与')
  }
  finally {
    rankBusy.value = false
    refresh()
  }
}

/* ── 业务信封错误本页提示；401/403（handlers 已全局提示）不重复 toast ── */

function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/* ── 导航 ── */

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/index/index' })
}

/* ── 添加好友（头部 + → 弹层输 user_id） ── */

const showAdd = ref(false)
const addUid = ref('')
const adding = ref(false)

function openAdd() {
  addUid.value = ''
  showAdd.value = true
}

async function onAdd() {
  const uid = Number(addUid.value)
  if (!Number.isInteger(uid) || uid <= 0) {
    toast.error('请输入正确的用户 ID')
    return
  }
  if (adding.value)
    return
  adding.value = true
  try {
    await addFriend(uid)
    toast.success('已添加好友')
    showAdd.value = false
    refresh()
  }
  catch (err) {
    // 30109 自加 / 30108 已被拉黑 / 40004 用户不存在：msg 由后端信封透出
    toastApiError(err)
  }
  finally {
    adding.value = false
  }
}

/* ── 删除 / 拉黑 / 解除 ── */

const deleting = ref(false)

async function onDelete(row: FriendItem) {
  if (deleting.value)
    return
  // 锁在 confirm 之前：弹窗停留期间再点不重入（同战绩页模式）
  deleting.value = true
  try {
    try {
      await dialog.confirm({
        title: '删除好友',
        msg: `删除与「${row.nickname || '该牌友'}」的好友关系，双方榜单不再互相可见，确认删除？`,
        confirmButtonText: '确认删除',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    await removeFriend(row.user_id)
    toast.success('已删除')
    refresh()
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    deleting.value = false
  }
}

async function onBlack(row: FriendItem) {
  try {
    await setBlack(row.user_id, true)
    toast.success('已拉黑')
    refresh()
  }
  catch (err) {
    toastApiError(err)
  }
}

async function onUnblack(row: FriendItem) {
  try {
    await setBlack(row.user_id, false)
    toast.success('已解除拉黑')
    refresh()
  }
  catch (err) {
    toastApiError(err)
  }
}

/* ── 生命周期 ── */

onShow(() => {
  if (!authStore.user)
    void authStore.fetchMe().catch(() => {})
  refresh()
})
</script>

<template>
  <view
    class="friends-page"
    @click="closeOutside"
  >
    <!-- 自绘 nav（同战绩页）；右侧「+」进添加好友弹层 -->
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
        好友排名
        <text class="nsub">
          内页左滑可删除、拉黑
        </text>
      </view>
      <view class="side right">
        <text
          class="add-btn"
          @click.stop="openAdd"
        >
          ＋
        </text>
      </view>
    </view>

    <!-- 汇总：加入排名开关 + 好友数 -->
    <view class="section">
      <view class="rrow sum">
        <view class="sum-text">
          <text class="rn">
            加入排名
          </text>
          <text class="rsub">
            关闭后不参与好友榜单，互相不可见 · 好友 {{ friendCount }}
          </text>
        </view>
        <wd-switch
          v-model="rankOn"
          :disabled="rankBusy"
          @change="onRankChange"
        />
      </view>
    </view>

    <!-- 榜单 -->
    <view class="list">
      <FriendRow
        v-for="(row, idx) in rows"
        :key="row.user_id"
        class="row"
        :item="row"
        :rank="idx + 1"
        :self="row.user_id === myUserId"
        @delete="onDelete(row)"
        @black="onBlack(row)"
        @unblack="onUnblack(row)"
      />
      <text
        v-if="!rows.length && !loading && bootstrapped"
        class="empty"
      >
        还没有好友，点右上角 ＋ 添加
      </text>
      <text
        v-if="loading"
        class="foot"
      >
        加载中…
      </text>
    </view>

    <view class="grow" />
    <text class="cap">
      左滑删除 / 拉黑好友 ·「· 我」为自己的名次<br>总分与胜率来自战绩结算
    </text>

    <!-- 添加好友弹层：输对方 user_id -->
    <wd-popup
      v-model="showAdd"
      position="bottom"
      closable
      :safe-area-inset-bottom="true"
      custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    >
      <view class="add-popup">
        <text class="ap-title">
          添加好友
        </text>
        <text class="ap-sub">
          输入对方的用户 ID，互加后榜单互相可见
        </text>
        <wd-input
          v-model="addUid"
          class="ap-input"
          type="number"
          :maxlength="12"
          clearable
          placeholder="对方用户 ID"
        />
        <button
          class="ap-btn"
          :disabled="adding"
          @click="onAdd"
        >
          {{ adding ? '添加中…' : '添加' }}
        </button>
      </view>
    </wd-popup>
  </view>
</template>

<style lang="scss" scoped>
.friends-page {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(env(safe-area-inset-top, 0px) + 6px) 0 calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav（同战绩页） ── */
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

.add-btn {
  color: var(--brass);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 26px;
  font-weight: 300;
  line-height: 1;
}

/* ── 汇总开关卡（设计 08 .rrow 变体） ── */
.section {
  flex: none;
  padding: 10px 20px 0;
}

.rrow.sum {
  display: flex;
  box-sizing: border-box;
  align-items: center;
  gap: 11px;
  padding: 13px 14px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 12px;
  background: var(--ink-700);
}

.sum-text {
  min-width: 0;
  flex: 1;
}

.rrow .rn {
  display: block;
  font-size: 14px;
}

.rrow .rsub {
  display: block;
  margin-top: 1px;
  color: var(--dim);
  font-size: 10.5px;
}

/* ── 榜单列表 ── */
.list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 9px;
  padding: 12px 20px 0;
}

.list .row {
  display: block;
}

.empty,
.foot {
  display: block;
  padding: 24px 0 8px;
  color: var(--dim);
  font-size: 12px;
  text-align: center;
}

.grow {
  flex: 1;
}

.cap {
  flex: none;
  padding: 12px 20px 0;
  color: var(--dim);
  font-size: 11px;
  line-height: 1.8;
  text-align: center;
}

/* ── 添加好友弹层 ── */
.add-popup {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  padding: 18px 20px calc(env(safe-area-inset-bottom, 0px) + 16px);
}

.ap-title {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.ap-sub {
  margin-top: 4px;
  color: var(--dim);
  font-size: 11px;
}

.ap-input {
  margin-top: 14px;
  border-radius: 12px;
  background: var(--ink-600);
}

.ap-btn {
  display: flex;
  height: 44px;
  align-items: center;
  justify-content: center;
  margin-top: 16px;
  border: none;
  border-radius: 12px;
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  color: var(--ink-900);
  font-size: 15px;
  font-weight: 600;

  &::after {
    border: none;
  }

  &[disabled] {
    opacity: 0.6;
  }
}
</style>
