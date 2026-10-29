<script setup lang="ts">
/**
 * 战绩页 · 房间记录 + 左滑操作（视觉源：desgin/preview/miniapp-breakout.html 07；Task 10 整体重写占位页）
 *
 * - stat-strip：stat → 总局数（win+lose+draw）/ 胜率百分 / 累计净成绩（ScoreText）
 * - 筛选 chip：「全部」选中；玩法 chip 置灰只展示（决策 #8，无功能过滤）
 * - 列表：myRecords 分页（onReachBottom 续拉）；行=日期 + ±分 + 结算类型文案
 * - 左滑（RecordSwipable）：流水 → popup 展示 recordDetail（序号+±分，无昵称）；
 *   排行 → 同数据按分降序；删除 → dialog 二次确认 → deleteRecord → 刷新列表与统计
 * - 「自定义统计」置灰「开发中」
 * - 业务错误本页 toast（ApiError 非 401/403）；401 由 handlers 全局清态跳登录
 * - 色值零硬编码：全走主题 token 与全局工具类（.num/.win-n/.lose-n）+ scoped var(--brass)
 * - 文案红线：全页文案只允许记分/结算词汇（锁定合规语除外）
 */
import type { RecordItem } from '@/api/scorecard/types'
import { useQueue } from '@wot-ui/ui'
import { ApiError } from '@/api/core/handlers'
import { deleteRecord, myRecords, recordDetail, stat } from '@/api/scorecard/record'
import RecordSwipable from '@/components/scorecard/RecordSwipable.vue'
import ScoreText from '@/components/scorecard/ScoreText.vue'
import { useSystemBar } from '@/composables/useSystemBar'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
// 必须带 'globalDialog' selector，共享 ref 由 App.ku.vue（global-ku-root）provide：
// 裸 useDialog() inject 不到该 key 会自建孤儿 ref，confirm 永不弹且 promise 挂起。
const dialog = useDialog('globalDialog')
const { closeOutside } = useQueue()
const { pageStyle } = useSystemBar()

/* ── 顶部统计条 ── */

const statResult = ref<{ total_score: number, win: number, lose: number, draw: number, win_rate: number } | null>(null)

const totalCount = computed(() => {
  const s = statResult.value
  return s ? s.win + s.lose + s.draw : 0
})
/** 胜率百分：win_rate 为 0~1 浮点（分母 0 后端给 0）。 */
const winRateText = computed(() => `${Math.round((statResult.value?.win_rate ?? 0) * 100)}%`)

/* ── 战绩列表分页 ── */

const PAGE_SIZE = 10

const records = ref<RecordItem[]>([])
const total = ref(0)
const page = ref(1)
const loading = ref(false)
/** 首屏骨架（首拉未回时 true，控制空态文案不闪现）。 */
const bootstrapped = ref(false)

async function loadRecords(append: boolean) {
  if (loading.value)
    return
  if (append && records.value.length >= total.value)
    return
  loading.value = true
  try {
    const p = append ? page.value + 1 : 1
    const res = await myRecords({ page: p, size: PAGE_SIZE })
    page.value = p
    total.value = res.total
    records.value = append ? [...records.value, ...res.list] : res.list
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

/** 刷新整页数据（删除后 / onShow）。 */
function refresh() {
  void loadStat()
  void loadRecords(false)
}

/** 触底续拉。 */
onReachBottom(() => {
  if (records.value.length < total.value)
    void loadRecords(true)
})

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

/* ── 流水 / 排行弹层（recordDetail 全房终局快照，共用一套行） ── */

const showPopup = ref(false)
const popupTitle = ref('房间流水')
const popupRows = ref<RecordItem[]>([])
const popupLoading = ref(false)

/** 流水：接口原序（全房每人一行，序号仅作计数）；排行：按分降序，rank=名次。 */
async function openPopup(recordId: number, mode: 'flow' | 'rank') {
  popupTitle.value = mode === 'rank' ? '本局排行' : '房间流水'
  popupRows.value = []
  popupLoading.value = true
  showPopup.value = true
  try {
    const rows = await recordDetail(recordId)
    popupRows.value = mode === 'rank' ? [...rows].sort((a, b) => b.score - a.score) : rows
  }
  catch (err) {
    toastApiError(err)
    showPopup.value = false
  }
  finally {
    popupLoading.value = false
  }
}

/* ── 删除 ── */

const deleting = ref(false)

async function onDelete(record: RecordItem) {
  if (deleting.value)
    return
  // 锁在 confirm 之前：弹窗停留期间再点不重入（同结算页模式）
  deleting.value = true
  try {
    try {
      await dialog.confirm({
        title: '删除战绩',
        msg: '删除后该条战绩不可恢复，确认删除？',
        confirmButtonText: '确认删除',
        cancelButtonText: '再想想',
      })
    }
    catch {
      return
    }
    await deleteRecord(record.id)
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

/* ── 生命周期 ── */

onShow(() => {
  refresh()
})
</script>

<template>
  <view
    class="records-page"
    :style="pageStyle"
    @click="closeOutside"
  >
    <!-- 自绘 nav（同结算页） -->
    <view class="nav">
      <view
        class="side"
        @click="goBack"
      >
        <wd-icon
          css-icon
          class="navicon"
          name="i-carbon-chevron-left"
        />
      </view>
      <view class="ntitle">
        战绩
        <text class="nsub">
          房间记录 · 左滑有流水 / 排行 / 删除
        </text>
      </view>
      <view class="side right">
        <!-- #ifndef MP-WEIXIN -->
        <view class="capsule">
          <text class="cap-dots">
            •••
          </text>
          <view class="sep" />
          <view class="cap-c2">
            <view class="ring" />
          </view>
        </view>
        <!-- #endif -->
      </view>
    </view>

    <!-- 统计条 -->
    <view class="stat-strip">
      <view class="stat-chip">
        <text class="v num">
          {{ totalCount }}
        </text>
        <text class="k">
          总局数
        </text>
      </view>
      <view class="stat-chip">
        <text class="v num brass">
          {{ winRateText }}
        </text>
        <text class="k">
          胜率
        </text>
      </view>
      <view class="stat-chip">
        <view class="brass">
          <ScoreText
            class="v"
            :value="statResult?.total_score ?? 0"
          />
        </view>
        <text class="k">
          累计净成绩
        </text>
      </view>
    </view>

    <!-- 筛选 chip：全部有效，玩法置灰（决策 #8） -->
    <view class="chips">
      <view class="fchip on">
        全部
      </view>
      <view class="fchip off">
        <view
          class="dotc"
          style="--fc: var(--vermillion)"
        />
        麻将
      </view>
      <view class="fchip off">
        <view
          class="dotc"
          style="--fc: var(--jade-hi)"
        />
        掼蛋
      </view>
      <view class="fchip off">
        <view
          class="dotc"
          style="--fc: var(--indigo-hi)"
        />
        斗地主
      </view>
    </view>

    <!-- 战绩列表 -->
    <view class="list">
      <RecordSwipable
        v-for="r in records"
        :key="r.id"
        class="row"
        :record="r"
        @detail="openPopup(r.id, 'flow')"
        @rank="openPopup(r.id, 'rank')"
        @delete="onDelete(r)"
      />
      <text
        v-if="!records.length && !loading && bootstrapped"
        class="empty"
      >
        暂无战绩，结算过的房间会出现在这里
      </text>
      <text
        v-if="loading"
        class="foot"
      >
        加载中…
      </text>
      <text
        v-else-if="records.length && records.length >= total"
        class="foot"
      >
        没有更多了
      </text>
    </view>

    <!-- 自定义统计入口：置灰「开发中」 -->
    <view class="stat-entry">
      <view class="adslot">
        <text class="devmark">
          开发中
        </text>
        <text class="txt">
          自定义统计：按月份 / 年份 / 具体记录 →
        </text>
      </view>
      <text class="cap">
        左滑房间记录可看流水 / 排行 / 删除
      </text>
    </view>

    <!-- 流水 / 排行弹层（序号 + ±分，无昵称） -->
    <wd-popup
      v-model="showPopup"
      position="bottom"
      closable
      :safe-area-inset-bottom="true"
      custom-style="border-radius: 20px 20px 0 0; background: var(--ink-700);"
    >
      <view class="detail-popup">
        <text class="dp-title">
          {{ popupTitle }}
        </text>
        <text class="dp-cap">
          共 {{ popupRows.length }} 人 · 按本局分数
        </text>
        <scroll-view
          class="dp-scroll"
          scroll-y
        >
          <view
            v-for="(row, idx) in popupRows"
            :key="row.user_id"
            class="drow"
          >
            <text class="rk num">
              {{ idx + 1 }}
            </text>
            <text class="who">
              第 {{ idx + 1 }} 位
            </text>
            <ScoreText
              class="rs"
              :value="row.score"
              :tone="row.score > 0 ? 'win' : (row.score < 0 ? 'lose' : 'none')"
            />
          </view>
          <text
            v-if="popupLoading"
            class="dp-foot"
          >
            加载中…
          </text>
          <text
            v-else-if="!popupRows.length"
            class="dp-foot"
          >
            暂无明细
          </text>
        </scroll-view>
      </view>
    </wd-popup>
  </view>
</template>

<style lang="scss" scoped>
.records-page {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 0 calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav（同结算页） ── */
.nav {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: space-between;
  padding: 6px 16px 8px;
  /* 小程序：右侧让位微信真胶囊 */
  padding-right: var(--nav-right-pad, 16px);
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

/* ── 统计条（设计 07 .stat-strip） ── */
.stat-strip {
  display: flex;
  flex: none;
  gap: 10px;
  padding: 14px 20px 0;
}

.stat-chip {
  display: flex;
  min-width: 0;
  flex: 1;
  flex-direction: column;
  padding: 10px 8px;
  border: 1px solid var(--on-ink-a04);
  border-radius: 14px;
  background: var(--ink-700);
  text-align: center;
}

.stat-chip .v {
  font-size: 23px;
  font-weight: 600;
  line-height: 1.1;
}

.stat-chip .k {
  margin-top: 3px;
  color: var(--dim);
  font-size: 10.5px;
}

/* 黄铜数字（胜率 / 累计净成绩）：token 色经继承注入子组件 */
.stat-chip .brass {
  color: var(--brass);
}

/* ── 筛选 chip（设计 07 .fchip；玩法置灰） ── */
.chips {
  display: flex;
  flex: none;
  gap: 8px;
  overflow: hidden;
  padding: 14px 20px 0;
}

.fchip {
  display: inline-flex;
  height: 32px;
  flex: none;
  align-items: center;
  padding: 0 15px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
  color: var(--dim);
  font-size: 13px;
}

.fchip.on {
  border-color: var(--brass);
  background: color-mix(in srgb, var(--brass) 10%, transparent);
  color: var(--brass);
  font-weight: 500;
  opacity: 1;
}

.fchip.off {
  opacity: 0.5;
}

.fchip .dotc {
  width: 8px;
  height: 8px;
  margin-right: 6px;
  border-radius: 50%;
  background: var(--fc, transparent);
}

/* ── 列表 ── */
.list {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 10px;
  padding: 14px 20px 0;
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

/* ── 自定义统计入口（置灰「开发中」） ── */
.stat-entry {
  flex: none;
  padding: 16px 20px 0;
}

.stat-entry .adslot {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 16px 13px;
  border: 1px solid var(--brass-a50);
  border-radius: 14px;
  background: var(--on-ink-a04);
  opacity: 0.6;
}

.stat-entry .devmark {
  display: inline-flex;
  flex: none;
  align-items: center;
  padding: 3px 9px;
  border: 1px dashed var(--brass-a50);
  border-radius: 6px;
  color: var(--brass);
  font-size: 11px;
}

.stat-entry .txt {
  color: var(--on-ink);
  font-size: 13px;
}

.stat-entry .cap {
  display: block;
  margin-top: 9px;
  color: var(--dim);
  font-size: 12px;
  text-align: center;
}

/* ── 流水 / 排行弹层 ── */
.detail-popup {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
  padding: 18px 0 calc(env(safe-area-inset-bottom, 0px) + 8px);
}

.dp-title {
  padding: 0 20px;
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.dp-cap {
  padding: 4px 20px 10px;
  color: var(--dim);
  font-size: 11px;
}

.dp-scroll {
  height: 55vh;
}

.drow {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 11px 20px;
  border-bottom: 1px solid var(--on-ink-a04);
}

.drow .rk {
  width: 22px;
  flex: none;
  color: var(--dim);
  font-size: 15px;
  text-align: center;
}

.drow:first-of-type .rk {
  color: var(--win-dark);
  font-weight: 700;
}

.drow .who {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  color: var(--on-ink);
  font-size: 14px;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.drow .rs {
  flex: none;
  font-size: 17px;
  font-weight: 600;
}

.dp-foot {
  display: block;
  padding: 10px 0 6px;
  color: var(--dim);
  font-size: 11px;
  text-align: center;
}
</style>
