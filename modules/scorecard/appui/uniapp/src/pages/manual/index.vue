<script setup lang="ts">
import PlayCard from '@/components/scorecard/PlayCard.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useScorecardAuthStore } from '@/store/scorecard/auth'
/**
 * 使用手册页 · 设计 06（Task 15 整体重写占位页）
 *
 * - 静态长页：完整转写 breakout 06 八张花色卡章节文案（①重要提示 ②结算·退房·历史
 *   ③茶饭设置 ④房间语音三类+5 档 ⑤好友排名 ⑥广告说明/功能优化/需求反馈），逐字对稿
 * - 章节头：PlayCard 花色角标（♥♠♦♣ + 序号），条纹/角标红按组件锁定规则
 * - 语音 5 档文案与 store/scorecard/voice.ts VOICE_MODE_ORDER/LABELS 一致：
 *   全部关闭 → 全部播放 → 娱乐关闭 → 流水关闭 → 操作播放（默认操作播放）
 * - 版本号取 package.json version（import pkg，非硬编码）
 * - 自定义 nav（同结算/战绩页）；底部「返回房间」= goBack（栈深 1 时回首页）
 * - 色值零硬编码：全走主题 token；合规红句为产品锁定语，逐字保留
 * - 文案红线：仅记分语汇；「不能涉及金钱」为合规锁定例外句
 */
import pkg from '../../../package.json'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const authStore = useScorecardAuthStore()
const { pageStyle } = useSystemBar()

/** package.json version（设计稿 5.3.1 为源稿示意值，实现取真实版本号）。 */
const version = pkg.version

/** 欢迎行：昵称取登录态（未登录/空昵称兜底「牌友」），头像圆用首字。 */
const nickname = computed(() => authStore.user?.nickname || '牌友')
const avatarLetter = computed(() => nickname.value.slice(0, 1).toUpperCase())

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/index/index' })
}
</script>

<template>
  <view
    class="manual-page"
    :style="pageStyle"
  >
    <!-- 自绘 nav（同结算/战绩页） -->
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
        使用手册
        <text class="nsub">
          打牌记账 · 当前版本 {{ version }}
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

    <!-- 欢迎行（源稿 profile row） -->
    <view class="welcome">
      <view class="avatar">
        <text class="av-letter">
          {{ avatarLetter }}
        </text>
      </view>
      <view class="w-main">
        <view class="w-name-row">
          <text class="w-name">
            {{ nickname }}
          </text>
          <text class="w-hi">
            欢迎阅读
          </text>
        </view>
        <text class="w-sub">
          操作说明 · 细微技巧 · 隐藏功能 · 反馈解决
        </text>
      </view>
    </view>

    <!-- 01 重要提示 -->
    <view
      class="mcard"
      style="--sc: var(--vermillion)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♥"
          rank="01"
        />
        <text class="man-t">
          重要提示
        </text>
      </view>
      <view class="man-b">
        <view class="li red">
          <text>“打牌记账”是记账、计分工具 — 不能涉及金钱，禁止用于违法行为</text>
        </view>
        <view class="li">
          <text>需要帮助：微信客服留言（人多请耐心等待，24h 内回复）</text>
        </view>
        <view class="li">
          <text>
            <text class="b">
              网络
            </text>：保持网络良好、亮屏体验最佳（息屏会断小程序网络）；数据不实时<text class="b">
              下拉刷新
            </text>，或右上角三点 → 重新进入小程序
          </text>
        </view>
      </view>
    </view>

    <!-- 02 结算 · 退房 · 历史记录 -->
    <view
      class="mcard"
      style="--sc: var(--on-ink)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♠"
          rank="02"
        />
        <text class="man-t">
          结算 · 退房 · 历史记录
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>
            <text class="b">
              结算
            </text>：仅提供策略参考，可不按策略，<text class="grn">
              均线下自行处理
            </text>
          </text>
        </view>
        <view class="li">
          <text>
            <text class="b">
              退房
            </text>：只能自己退房，没有踢人；结算后点头像退出，数据为零可直接退；房间无操作 2 小时自动关闭
          </text>
        </view>
        <view class="li">
          <text>
            <text class="b">
              战绩
            </text>：首页点战绩 → 房间记录，<text class="b">
              左滑
            </text>有流水 / 排行 / 删除；好友排名内页左滑可删除、拉黑，点图标可清零
          </text>
        </view>
      </view>
    </view>

    <!-- 03 茶饭设置 -->
    <view
      class="mcard"
      style="--sc: var(--brass)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♦"
          rank="03"
        />
        <text class="man-t">
          茶饭设置
        </text>
        <text class="flag">
          完成
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>
            房间点茶/饭弹层配置：目标 + 抽取百分比；<text class="orange">
              目标 &gt; 0 则开关开启
            </text>
          </text>
        </view>
        <view class="li">
          <text>
            <text class="orange">
              方式一
            </text>：抽取百分比 &gt; 0% 时，每次支出记账，赢方按比例另行记为茶饭
          </text>
        </view>
        <view class="li">
          <text>
            <text class="orange">
              方式二
            </text>：房间左下角支出按钮，直接 DIY 数值记为茶饭
          </text>
        </view>
      </view>
    </view>

    <!-- 04 房间语音 -->
    <view
      class="mcard"
      style="--sc: var(--jade-hi)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♣"
          rank="04"
        />
        <text class="man-t">
          房间语音
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>
            语音分三类：娱乐 / 流水 / 房间动态；点头像弹窗右侧喇叭，<text class="b">
              5 档循环
            </text>：全部关闭 → 全部播放 → 娱乐关闭 → 流水关闭 → 操作播放
          </text>
        </view>
        <view class="li">
          <text>
            默认：<text class="grn">
              操作播放
            </text>（人员进出、结算）；开房人默认全开
          </text>
        </view>
      </view>
    </view>

    <!-- 05 好友排名 -->
    <view
      class="mcard"
      style="--sc: var(--on-ink)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♠"
          rank="05"
        />
        <text class="man-t">
          好友排名
        </text>
        <text class="flag">
          完成
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>内页左滑：删除好友、拉黑好友；可切换「不参与排名 / 加入排名」</text>
        </view>
      </view>
    </view>

    <!-- 06 添加广告说明 -->
    <view
      class="mcard"
      style="--sc: var(--vermillion)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♥"
          rank="06"
        />
        <text class="man-t">
          添加广告说明
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>
            非收费工具，服务器成本高，增加广告投放带来体验损失敬请谅解；<text class="orange">
              广告位尽量避开功能按钮，减少误入
            </text>
          </text>
        </view>
      </view>
    </view>

    <!-- 07 功能优化 -->
    <view
      class="mcard"
      style="--sc: var(--brass)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♦"
          rank="07"
        />
        <text class="man-t">
          功能优化
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>部分机型房间信息/广播停止更新：与 WiFi 长连接被断有关，建议 4G；房间支持下滑刷新、检查网络；锁屏久后进入会提示确认刷新</text>
        </view>
      </view>
    </view>

    <!-- 08 需求反馈 -->
    <view
      class="mcard"
      style="--sc: var(--jade-hi)"
    >
      <view class="man-head">
        <PlayCard
          class="sec-pc"
          suit="♣"
          rank="08"
        />
        <text class="man-t">
          需求反馈
        </text>
        <text class="flag dev">
          开发中
        </text>
      </view>
      <view class="man-b">
        <view class="li">
          <text>战绩自定义统计：增加统计入口，可按月份 / 年份 / 具体记录统计</text>
        </view>
      </view>
    </view>

    <!-- 底部：返回房间 -->
    <view class="foot">
      <button
        class="btn bone"
        @click="goBack"
      >
        返回房间
      </button>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.manual-page {
  box-sizing: border-box;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

/* ── 自绘 nav（同结算/战绩页） ── */
.nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0 8px;
  /* 小程序：右侧让位微信真胶囊（页面根已有水平 16px，变量需扣掉避免双倍缩进） */
  padding-right: calc(var(--nav-right-pad, 16px) - 16px);
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
  font-size: 12px;
  text-align: center;
}

.capsule .sep {
  width: 1px;
  height: 16px;
  background: var(--on-ink-a28);
}

.cap-c2 {
  display: flex;
  width: 34px;
  height: 30px;
  align-items: center;
  justify-content: center;
}

.capsule .ring {
  width: 14px;
  height: 14px;
  border: 2px solid var(--on-ink);
  border-radius: 50%;
}

/* ── 欢迎行 ── */
.welcome {
  display: flex;
  gap: 12px;
  align-items: center;
  padding: 8px 4px 14px;
}

.welcome .avatar {
  display: flex;
  flex: none;
  width: 44px;
  height: 44px;
  align-items: center;
  justify-content: center;
  border: 2px solid var(--brass);
  border-radius: 50%;
  background: var(--ink-700);
  color: var(--brass);
}

.av-letter {
  font-size: 17px;
  font-weight: 500;
}

.w-main {
  min-width: 0;
}

.w-name-row {
  display: flex;
  gap: 6px;
  align-items: baseline;
}

.w-name {
  color: var(--on-ink);
  font-size: 15px;
  font-weight: 500;
}

.w-hi {
  color: var(--dim);
  font-size: 11px;
}

.w-sub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-size: 11px;
  line-height: 1.55;
}

/* ── 花色卡章节（源稿 .mcard：左色条 + PlayCard 章节头 + 要点列表） ── */
.mcard {
  position: relative;
  margin-bottom: 13px;
  padding: 14px 16px 14px 18px;
  overflow: hidden;
  border: 1px solid var(--on-ink-a04);
  border-radius: 18px;
  background: var(--ink-700);
}

.mcard::before {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  left: 0;
  width: 4px;
  background: var(--sc, var(--brass));
  opacity: 0.85;
}

.man-head {
  display: flex;
  gap: 10px;
  align-items: center;
  margin-bottom: 8px;
}

/* PlayCard 紧凑章节头（class 落组件根节点；尺寸本页收窄） */
.sec-pc {
  flex: none;
  width: 48px;
  height: 64px;
}

.sec-pc .idx {
  top: 7px;
  left: 7px;
}

.sec-pc .idx.br {
  top: auto;
  right: 7px;
  bottom: 7px;
  left: auto;
}

.man-t {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

.flag {
  margin-left: auto;
  padding: 1px 7px;
  border: 1px solid var(--jade-hi);
  border-radius: 4px;
  color: var(--jade-hi);
  font-size: 10.5px;
}

.flag.dev {
  border-color: var(--brass);
  color: var(--brass);
}

.man-b .li {
  position: relative;
  margin-bottom: 5px;
  padding-left: 14px;
  color: var(--on-ink);
  font-size: 12.5px;
  line-height: 1.75;
}

.man-b .li::before {
  content: '';
  position: absolute;
  top: 8px;
  left: 0;
  width: 5px;
  height: 5px;
  border-radius: 50%;
  background: var(--dim);
}

.man-b .li.red {
  color: var(--vermillion-hi);
  font-weight: 500;
}

/* 行内强调（源稿 .man-b b/.orange/.grn） */
.man-b .b {
  color: var(--on-ink);
  font-weight: 500;
}

.man-b .li .b {
  font-weight: 700;
}

.man-b .orange {
  color: var(--brass);
  font-weight: 500;
}

.man-b .grn {
  color: var(--jade-hi);
  font-weight: 500;
}

/* ── 底部返回 ── */
.foot {
  padding: 4px 4px 8px;
}
</style>
