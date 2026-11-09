<script setup lang="ts">
/**
 * 【我的】页 · 趣味方位模块唯一入口载体（设计 desgin/preview/integration.html 屏 02）
 *
 * - 迷你身份牌行 → ProfileGate edit（未登录先去登录，与首页点头像同语义）
 * - 常规分组：使用手册 / 客服（wd-button open-type=contact，同房间页筹码轨）/
 *   获取头像昵称（MP 官方 chooseAvatar/nickname 快捷通道 = ProfileGate 的 MP 分支，H5 隐藏）
 * - 「更多趣味小工具」折叠分组：趣味方位参考 / 工资到账计算器 / 日期计算器（占位）；fun_compass_enable 显式 false 时整组不渲染
 *   （useFunCompassConfig，spec 入口门控）；趣味方位参考 → pages/fun/compass/index
 * - 自绘 nav 沿用 manual 页模式；色值全走 token，无硬编码
 * - 语义类 mine-* 前缀（scoped 样式，避 UnoCSS 全局工具类撞名清单）
 */
import { stat } from '@/api/scorecard/record'
import ProfileGate from '@/components/scorecard/ProfileGate.vue'
import { useFunCompassConfig } from '@/composables/funCompass/useFunCompassConfig'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
import { MINE_GROUP_FOOT } from '@/utils/funCompass/copy'
import pkg from '../../../package.json'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
const authStore = useScorecardAuthStore()
const { pageStyle } = useSystemBar()
const { enabled } = useFunCompassConfig()

const isLoggedIn = computed(() => Boolean(authStore.token))
const version = pkg.version

const nickname = computed(() => authStore.user?.nickname || '牌友')
const avatarLetter = computed(() => nickname.value.slice(0, 1).toUpperCase())

/* ── 总积分（迷你身份牌右侧；登录态拉取，失败降级「—」） ── */

const totalScoreText = ref('—')

function formatDelta(score: number): string {
  if (score > 0)
    return `+${score}`
  if (score < 0)
    return `−${Math.abs(score)}`
  return '0'
}

async function loadStat() {
  if (!isLoggedIn.value) {
    totalScoreText.value = '—'
    return
  }
  try {
    const s = await stat()
    totalScoreText.value = formatDelta(s.total_score)
  }
  catch {
    totalScoreText.value = '—'
  }
}

onShow(() => {
  void loadStat()
})

/* ── 导航与资料 ── */

const showProfileEdit = ref(false)

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/index/index' })
}

/** 身份牌行 / 获取头像昵称行：已登录开编辑弹框，未登录去登录（与首页点头像一致）。 */
function onProfileRow() {
  if (!isLoggedIn.value) {
    router.push({ path: '/pages/login/index' })
    return
  }
  showProfileEdit.value = true
}

function onProfileSaved() {
  toast.success('资料已更新')
}

function goManual() {
  router.push({ path: '/pages/manual/index' })
}

function goCompass() {
  router.push({ path: '/pages/fun/compass/index' })
}

/** 工资到账计算器（fun-salary 变更）→ 分包 pages/fun/salary/index */
function goSalary() {
  router.push({ path: '/pages/fun/salary/index' })
}
</script>

<template>
  <view
    class="mine-page"
    :style="pageStyle"
  >
    <!-- 自绘 nav（同 manual/结算页） -->
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
        我的
        <text class="nsub">
          打牌记账 · 设置
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

    <!-- 迷你身份牌（骨白牌行）→ 修改资料 -->
    <view
      class="mine-idcard"
      @click="onProfileRow"
    >
      <view class="avatar">
        <text class="av-letter">
          {{ avatarLetter }}
        </text>
      </view>
      <view class="id-main">
        <text class="id-name">
          {{ isLoggedIn ? nickname : '未登录' }}
        </text>
        <text class="id-sub">
          {{ isLoggedIn ? '点击完善资料' : '登录后可开房记账' }}
        </text>
      </view>
      <view class="id-score">
        <text class="id-score-v">
          {{ totalScoreText }}
        </text>
        <text class="id-score-l">
          总积分
        </text>
      </view>
      <wd-icon
        css-icon
        class="chev"
        name="i-carbon-chevron-right"
      />
    </view>

    <!-- 常规分组 -->
    <view class="mine-group">
      <view
        class="g-row"
        @click="goManual"
      >
        <text class="r-t">
          使用手册
        </text>
        <wd-icon
          css-icon
          class="chev"
          name="i-carbon-chevron-right"
        />
      </view>

      <!-- 微信客服会话（同房间页筹码轨；open-type 在 H5 无平台能力，行仍可见） -->
      <view class="g-row">
        <wd-button
          variant="text"
          open-type="contact"
          block
          custom-class="mine-kefu-btn"
          custom-style="padding: 0; height: auto; line-height: 1; background: transparent; border: none; border-radius: 0; color: inherit; text-align: left;"
        >
          <view class="mine-kefu-inner">
            <text class="r-t">
              客服
            </text>
            <wd-icon
              css-icon
              class="chev"
              name="i-carbon-chevron-right"
            />
          </view>
        </wd-button>
      </view>
      <!-- #ifdef MP-WEIXIN -->
      <view
        class="g-row"
        @click="onProfileRow"
      >
        <text class="r-t">
          获取头像昵称
        </text>
        <wd-icon
          css-icon
          class="chev"
          name="i-carbon-chevron-right"
        />
      </view>
      <!-- #endif -->
    </view>

    <!-- 更多趣味小工具（fun_compass_enable=false 时整组不渲染） -->
    <view
      v-if="enabled"
      class="mine-group"
    >
      <view class="g-head">
        <text class="g-t">
          更多趣味小工具
        </text>
      </view>

      <view
        class="g-row fun-row"
        @click="goCompass"
      >
        <view class="fc-tile">
          <text class="fc-tile-t">
            東
          </text>
        </view>
        <view class="r-main">
          <text class="r-t">
            趣味方位参考
          </text>
          <text class="r-s">
            排八字 · 看罗盘 · 求个座
          </text>
        </view>
        <text class="badge-new">
          新
        </text>
        <wd-icon
          css-icon
          class="chev"
          name="i-carbon-chevron-right"
        />
      </view>

      <view
        class="g-row fun-row"
        @click="goSalary"
      >
        <view class="fc-tile y">
          <text class="fc-tile-t">
            ¥
          </text>
        </view>
        <view class="r-main">
          <text class="r-t">
            工资到账计算器
          </text>
          <text class="r-s">
            算日薪 · 看入账
          </text>
        </view>
        <wd-icon
          css-icon
          class="chev"
          name="i-carbon-chevron-right"
        />
      </view>

      <view class="g-row dim-row">
        <view class="r-main">
          <text class="r-t">
            日期计算器
          </text>
          <text class="r-s">
            敬请期待
          </text>
        </view>
      </view>

      <text class="g-foot">
        {{ MINE_GROUP_FOOT }}
      </text>
    </view>

    <text class="mine-ver">
      打牌记账 · v{{ version }}
    </text>

    <!-- 修改资料（与首页点头像同一弹框） -->
    <ProfileGate
      v-model:show="showProfileEdit"
      mode="edit"
      @saved="onProfileSaved"
    />
  </view>
</template>

<style lang="scss" scoped>
.mine-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  color: var(--on-ink);
  background: var(--ink-900);
  background-image: var(--spot);
}

/* ── 自绘 nav（同 manual 页） ── */
.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0 8px;

  /* 小程序：右侧让位微信真胶囊（根已水平 16px，变量扣掉避免双倍缩进） */
  padding-right: calc(var(--nav-right-pad, 16px) - 16px);
}

.nav .side {
  display: flex;
  align-items: center;
  min-width: 60px;
}

.nav .side.right {
  justify-content: flex-end;
}

.navicon {
  margin-right: 8px;
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  line-height: 1;
  color: var(--brass);
}

.ntitle {
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  color: var(--on-ink);
  text-align: center;
  letter-spacing: 0.06em;
}

.nsub {
  display: block;
  margin-top: 2px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  color: var(--dim);
  letter-spacing: 0.02em;
}

/* 胶囊占位（H5 装饰，对齐 manual 页） */
.capsule {
  display: flex;
  align-items: center;
  width: 88px;
  height: 32px;
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
}

.cap-dots {
  flex: 1;
  font-size: 12px;
  color: var(--on-ink);
  text-align: center;
}

.capsule .sep {
  width: 1px;
  height: 16px;
  background: var(--on-ink-a28);
}

.cap-c2 {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 30px;
}

.capsule .ring {
  width: 14px;
  height: 14px;
  border: 2px solid var(--on-ink);
  border-radius: 50%;
}

/* ── 迷你身份牌（骨白牌行） ── */
.mine-idcard {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 14px;
  margin: 8px 0 16px;
  color: var(--jet);
  background: var(--tile);
  border-radius: var(--r-tile);
  box-shadow: var(--shadow-tile);
}

.mine-idcard .avatar {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  color: var(--brass);
  background: var(--ink-700);
  border: 2px solid var(--brass);
  border-radius: 50%;
}

.av-letter {
  font-size: 17px;
  font-weight: 500;
}

.id-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.id-name {
  overflow: hidden;
  text-overflow: ellipsis;
  font-family: 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 700;
  color: var(--jet);
  white-space: nowrap;
}

.id-sub {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
  color: var(--jet);
  opacity: 0.55;
}

.id-score {
  display: flex;
  flex: none;
  flex-direction: column;
  align-items: flex-end;
}

.id-score-v {
  font-family: Oswald, sans-serif;
  font-size: 20px;
  font-weight: 600;
  color: var(--jet);
}

.id-score-l {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10px;
  color: var(--jet);
  opacity: 0.55;
}

.chev {
  flex: none;
  font-size: 18px;
  color: var(--jet);
  opacity: 0.4;
}

/* ── 深色分组卡 ── */
.mine-group {
  margin-bottom: 16px;
  overflow: hidden;
  background: var(--ink-700);
  border-radius: var(--r-tile);
}

.g-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 14px 4px;
}

.g-t {
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 700;
  color: var(--on-ink);
  letter-spacing: 0.02em;
}

.g-flag {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
  color: var(--dim);
}

.g-row {
  display: flex;
  gap: 12px;
  align-items: center;
  min-height: 52px;
  padding: 10px 14px;
  border-top: 1px solid var(--ink-600);
}

.g-head + .g-row {
  border-top: none;
}

.r-t {
  flex: 1;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15px;
  color: var(--on-ink);
}

.r-main {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.r-s {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  color: var(--dim);
}

/* 趣味方位参考 · 麻将牌片图标 */
.fc-tile {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  background: var(--tile);
  border-radius: 8px;
  box-shadow: 0 2px 0 rgb(0 0 0 / 50%);
}

.fc-tile-t {
  font-family: 'Noto Serif SC', serif;
  font-size: 18px;
  font-weight: 700;
  color: var(--vermillion);
}

/* 工资行：¥ 黄铜（区别于方位行的漆红東） */
.fc-tile.y .fc-tile-t {
  color: var(--brass);
}

.badge-new {
  flex: none;
  padding: 1px 7px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10px;
  font-weight: 500;
  color: var(--tile);
  background: var(--vermillion);
  border-radius: 999px;
}

.dim-row {
  opacity: 0.45;
}

/* 客服行：wd-button 拍平为普通行（block + custom-style 去默认），
   内层撑满让 r-t 的 flex:1 把 chev 推到右侧，对齐其它行 */
.mine-kefu-inner {
  display: flex;
  align-items: center;
  width: 100%;
}

/* wd-button 内部包裹层默认按内容收缩 + 居中，会让整行塌成一小截。
   须从页面元素锚定顶层 :deep（小程序端组件根节点不带页面 scope id，
   嵌套写法 `.cls { :deep(...) }` 在 MP 端匹配不到——同 PrivacyPopup 模式） */
:deep(.mine-kefu-btn .wd-button__text) {
  width: 100%;
}

:deep(.mine-kefu-btn .wd-button__content) {
  justify-content: flex-start;
}

.g-foot {
  display: block;
  padding: 8px 14px 12px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  color: var(--dim);
}

.mine-ver {
  display: block;
  padding: 4px 0 8px;
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
  color: var(--dim);
  text-align: center;
}
</style>
