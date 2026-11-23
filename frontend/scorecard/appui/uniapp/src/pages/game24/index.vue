<script setup lang="ts">
import type { Token } from '@/utils/game24/eval'
/**
 * 算24 pages/game24/index（openspec fun-minigames-24-huarong）
 *
 * - 有解发牌 dealCards()：初始 + 每题重发；纯本机，无存储无接口
 * - 拼式缓冲 steps：元素带来源（数字=cards 下标 / 'op' / 'paren'），
 *   按索引消耗牌——重复数字互不影响；⌫ 回退恢复对应牌
 * - 实时式子 formatExpr；提交 evalTokens + ratEq(_, {n:24,d:1})，
 *   非法式（null）与非 24 同走错误分支（toast + 式子红显，不清空可改）
 * - 看答案 findSolution → 式子区/toast 展示；理论上必有解，null 兜底换题
 * - 正解 toast + 0.8s 后换题清空；换一题重发；清空只清 tokens 恢复全部牌
 */
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { evalTokens, ratEq } from '@/utils/game24/eval'
import { dealCards, findSolution, formatExpr } from '@/utils/game24/solve'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const toast = useToast()
const { pageStyle } = useSystemBar()

/** 自绘 nav 返回：栈内回退（模板里直接写 uni 会 TS2339） */
function onBack() {
  uni.navigateBack()
}

/** 拼式步：token + 来源（数字=牌下标；运算符 'op'；括号 'paren'） */
interface Step {
  token: Token
  src: number | 'op' | 'paren'
}

/** 当前 4 张牌（有解） */
const cards = ref<number[]>(dealCards())
/** 拼式缓冲 */
const steps = ref<Step[]>([])
/** 看答案展示串（null=未展示；任意编辑后清除） */
const answerText = ref<string | null>(null)
/** 提交错误：式子短暂红显 */
const wrongFlash = ref(false)

const tokens = computed<Token[]>(() => steps.value.map(s => s.token))
const exprText = computed(() => formatExpr(tokens.value))
/** 已消耗牌下标（按索引，重复数字互不影响） */
const usedSet = computed(() => {
  const s = new Set<number>()
  for (const st of steps.value) {
    if (typeof st.src === 'number')
      s.add(st.src)
  }
  return s
})

/** 运算区六钮（展示用 ×÷，token 存 * /） */
const opBtns: { label: string, token: Token }[] = [
  { label: '+', token: '+' },
  { label: '−', token: '-' },
  { label: '×', token: '*' },
  { label: '÷', token: '/' },
  { label: '(', token: '(' },
  { label: ')', token: ')' },
]

/** 牌面展示：1→A、11–13→J/Q/K */
function cardFace(n: number): string {
  if (n === 1)
    return 'A'
  if (n === 11)
    return 'J'
  if (n === 12)
    return 'Q'
  if (n === 13)
    return 'K'
  return String(n)
}

let dealTimer: ReturnType<typeof setTimeout> | null = null
let flashTimer: ReturnType<typeof setTimeout> | null = null

function clearAnswer() {
  answerText.value = null
}

/** 点数牌：已消耗禁点；按索引入缓冲 */
function onPickCard(i: number) {
  if (usedSet.value.has(i))
    return
  clearAnswer()
  steps.value.push({ token: cards.value[i], src: i })
}

/** 运算符/括号入缓冲 */
function onOp(token: Token) {
  clearAnswer()
  const src: 'op' | 'paren' = token === '(' || token === ')' ? 'paren' : 'op'
  steps.value.push({ token, src })
}

/** ⌫ 回退末位：数字步移除后 usedSet 自动恢复该牌 */
function onBackspace() {
  clearAnswer()
  steps.value.pop()
}

/** 提交判定：24 → toast + 0.8s 换题；否则红显不清空 */
function onSubmit() {
  clearAnswer()
  const val = evalTokens(tokens.value)
  if (val !== null && ratEq(val, { n: 24, d: 1 })) {
    toast.success('正解！')
    if (dealTimer)
      clearTimeout(dealTimer)
    dealTimer = setTimeout(() => {
      dealTimer = null
      cards.value = dealCards()
      steps.value = []
      wrongFlash.value = false
    }, 800)
    return
  }
  // 非法式（null）与结果非 24 同走错误分支：可改不清空
  toast.error(val === null ? '算式不合法' : '还不等于 24，再试试')
  wrongFlash.value = true
  if (flashTimer)
    clearTimeout(flashTimer)
  flashTimer = setTimeout(() => {
    wrongFlash.value = false
    flashTimer = null
  }, 900)
}

/** 看答案：findSolution 展示；null 兜底换题 */
function onShowAnswer() {
  const sol = findSolution(cards.value)
  if (sol === null) {
    toast.warning('无解，已换一题')
    cards.value = dealCards()
    steps.value = []
    clearAnswer()
    wrongFlash.value = false
    return
  }
  answerText.value = formatExpr(sol)
  toast.success(`答案：${answerText.value}`)
}

/** 换一题：重发 + 清空 */
function onNewRound() {
  if (dealTimer) {
    clearTimeout(dealTimer)
    dealTimer = null
  }
  cards.value = dealCards()
  steps.value = []
  clearAnswer()
  wrongFlash.value = false
}

/** 清空：只清 tokens，恢复全部牌 */
function onClear() {
  steps.value = []
  clearAnswer()
  wrongFlash.value = false
}

onUnmounted(() => {
  if (dealTimer)
    clearTimeout(dealTimer)
  if (flashTimer)
    clearTimeout(flashTimer)
})
</script>

<template>
  <view
    class="g24-screen"
    :style="pageStyle"
  >
    <ModNav
      title="算24"
      sub="趣味小工具 · 仅供娱乐"
      @back="onBack"
    />

    <view class="g24-content">
      <!-- 式子区：实时 formatExpr；看答案覆盖展示；错误红显 -->
      <view
        class="g24-expr"
        :class="{ 'g24-expr--wrong': wrongFlash, 'g24-expr--answer': answerText !== null }"
      >
        <text
          v-if="answerText !== null"
          class="g24-expr-t"
        >
          {{ answerText }}
        </text>
        <text
          v-else-if="steps.length > 0"
          class="g24-expr-t"
        >
          {{ exprText }}
        </text>
        <text
          v-else
          class="g24-expr-t g24-expr-t--ph"
        >
          点选数字与运算符
        </text>
        <text
          v-if="answerText !== null"
          class="g24-expr-cap"
        >
          参考答案
        </text>
      </view>

      <!-- 牌区：4 张数牌，已点选用过置灰禁点 -->
      <view class="g24-cards">
        <view
          v-for="(c, i) in cards"
          :key="i"
          class="g24-card"
          :class="{ 'g24-card--used': usedSet.has(i) }"
          hover-class="g24-card--hover"
          @click="onPickCard(i)"
        >
          <text class="g24-card-t">
            {{ cardFace(c) }}
          </text>
        </view>
      </view>

      <!-- 运算区：+ − × ÷ ( ) + ⌫ -->
      <view class="g24-ops">
        <view
          v-for="op in opBtns"
          :key="op.label"
          class="g24-op"
          hover-class="g24-pill--hover"
          @click="onOp(op.token)"
        >
          <text class="g24-op-t">
            {{ op.label }}
          </text>
        </view>
        <view
          class="g24-op g24-op--bs"
          hover-class="g24-pill--hover"
          @click="onBackspace"
        >
          <text class="g24-op-t">
            ⌫
          </text>
        </view>
      </view>

      <!-- 动作区：提交 / 看答案 / 换一题 / 清空 -->
      <view class="g24-actions">
        <view
          class="g24-btn g24-btn--primary"
          hover-class="g24-pill--hover"
          @click="onSubmit"
        >
          <text class="g24-btn-t g24-btn-t--primary">
            提交
          </text>
        </view>
        <view
          class="g24-btn"
          hover-class="g24-pill--hover"
          @click="onShowAnswer"
        >
          <text class="g24-btn-t">
            看答案
          </text>
        </view>
        <view
          class="g24-btn"
          hover-class="g24-pill--hover"
          @click="onNewRound"
        >
          <text class="g24-btn-t">
            换一题
          </text>
        </view>
        <view
          class="g24-btn"
          hover-class="g24-pill--hover"
          @click="onClear"
        >
          <text class="g24-btn-t">
            清空
          </text>
        </view>
      </view>

      <view class="g24-priv">
        <view class="g24-priv-dot" />
        <text class="g24-priv-t">
          本机即时判定 · 数据不上传
        </text>
      </view>
    </view>

    <view class="g24-footer">
      <text class="g24-footer-t">
        用四张牌凑 24 · 仅供娱乐参考
      </text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.g24-screen {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 0 calc(env(safe-area-inset-bottom, 0px));
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
  overflow: hidden;
}

/* 内容区滚动（nav/底注钉住） */
.g24-content {
  flex: 1;
  min-height: 0;
  padding: 6px 16px 12px;
  overflow-y: auto;
}

/* 宽屏收进 480 手机栏（box-sizing 显式，防窄屏溢出） */
.fcn-nav,
.g24-content,
.g24-footer {
  box-sizing: border-box;
  width: 100%;
  max-width: 480px;
  margin-inline: auto;
}

/* ── 式子区 ── */
.g24-expr {
  position: relative;
  display: flex;
  min-height: 72px;
  align-items: center;
  justify-content: center;
  margin-bottom: 14px;
  padding: 16px 14px 18px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
  transition: box-shadow 0.2s ease;
}

.g24-expr--wrong {
  box-shadow: 0 0 0 2px rgba(214, 84, 72, 0.85), var(--shadow-tile);
}

.g24-expr--answer {
  box-shadow: 0 0 0 2px rgba(201, 162, 75, 0.75), var(--shadow-tile);
}

.g24-expr-t {
  font-family: 'Oswald', sans-serif;
  font-size: 26px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-align: center;
  word-break: break-all;
}

.g24-expr-t--ph {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14px;
  font-weight: 400;
  opacity: 0.45;
}

.g24-expr-cap {
  position: absolute;
  right: 12px;
  bottom: 6px;
  color: var(--brass);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11px;
}

/* ── 牌区 ── */
.g24-cards {
  display: flex;
  gap: 10px;
  justify-content: space-between;
  margin-bottom: 14px;
}

.g24-card {
  display: flex;
  flex: 1;
  max-width: 88px;
  aspect-ratio: 3 / 4;
  align-items: center;
  justify-content: center;
  border-radius: 14px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.g24-card--used {
  background: var(--ink-700);
  box-shadow: none;
  opacity: 0.38;
  pointer-events: none;
}

.g24-card--hover {
  opacity: 0.82;
  transform: scale(0.97);
}

.g24-card--used.g24-card--hover {
  opacity: 0.38;
}

.g24-card-t {
  color: var(--brass);
  font-family: 'Oswald', sans-serif;
  font-size: 34px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.g24-card--used .g24-card-t {
  color: var(--dim);
}

/* ── 运算区 ── */
.g24-ops {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 10px;
  margin-bottom: 16px;
}

.g24-op {
  display: flex;
  height: 52px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--ink-700);
  border: 1px solid var(--on-ink-a28);
}

.g24-op--bs {
  background: transparent;
}

.g24-pill--hover {
  opacity: 0.82;
  transform: scale(0.97);
}

.g24-op-t {
  color: var(--on-ink);
  font-family: 'Oswald', sans-serif;
  font-size: 22px;
  font-weight: 600;
}

/* ── 动作区 ── */
.g24-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 14px;
}

.g24-btn {
  display: flex;
  height: 46px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: var(--ink-700);
  border: 1px solid var(--on-ink-a28);
}

.g24-btn--primary {
  background: var(--brass);
  border-color: transparent;
  grid-column: 1 / -1;
}

.g24-btn-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 500;
  letter-spacing: 0.05em;
}

.g24-btn-t--primary {
  color: var(--ink-900);
  font-weight: 700;
}

/* ── 本机提示 ── */
.g24-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
}

.g24-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade);
}

.g24-priv-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}

/* ── 底注（钉底） ── */
.g24-footer {
  flex: none;
  padding: 10px 16px 8px;
  text-align: center;
}

.g24-footer-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}
</style>
