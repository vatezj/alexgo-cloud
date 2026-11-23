<script setup lang="ts">
import type { Dir, Piece } from '@/utils/huarong/board'
/**
 * 华容道 pages/huarong/index（openspec fun-minigames-24-huarong）
 *
 * - 经典「横刀立马」4×5 单局（INITIAL_PIECES），无关卡无计时无接口
 * - 点块选中 → 仅渲染 movableDirs 可动方向箭头 → 点箭头 movePiece 重赋值 + 计步
 * - 点空白/再点同块取消选中；棋盘状态只经 movePiece 重赋值，勿就地改单枚
 * - 每次移动后 isWin 判胜：toast 庆祝 + 更优才写 huarong:best + 锁定到重玩
 * - 头部：步数 / 最佳（null 显示「—」）/ 重置（回初始、清零、解锁）
 */
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import {
  canMove,
  COLS,
  INITIAL_PIECES,
  isWin,
  movableDirs,
  movePiece,
  ROWS,
} from '@/utils/huarong/board'

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

/* ── 状态 ── */

/** 棋盘棋子（浅拷贝枚对象，不与 INITIAL_PIECES 共享可变引用） */
const pieces = ref<Piece[]>(INITIAL_PIECES.map(p => ({ ...p })))
/** 当前步数 */
const steps = ref(0)
/** 最佳步数（null=无记录，显示「—」） */
const best = ref<number | null>(readBest())
/** 选中棋子 id（null=未选中） */
const selectedId = ref<string | null>(null)
/** 通关锁定（直到重置/重玩） */
const won = ref(false)

function readBest(): number | null {
  const raw = uni.getStorageSync('huarong:best')
  return typeof raw === 'number' && Number.isFinite(raw) ? raw : null
}

const selectedPiece = computed<Piece | null>(() =>
  pieces.value.find(p => p.id === selectedId.value) ?? null)

/** 选中棋子当前可动方向（箭头只渲染这些） */
const selDirs = computed<Dir[]>(() =>
  selectedId.value ? movableDirs(pieces.value, selectedId.value) : [])

/* ── 样式换算（4 列 5 行百分比定位，不溢出棋盘） ── */

/** 棋子绝对定位：格坐标 → 棋盘百分比，4px 缝隙 */
function pieceStyle(p: Piece): Record<string, string> {
  const gap = 4
  return {
    left: `calc(${(p.x / COLS) * 100}% + ${gap / 2}px)`,
    top: `calc(${(p.y / ROWS) * 100}% + ${gap / 2}px)`,
    width: `calc(${(p.w / COLS) * 100}% - ${gap}px)`,
    height: `calc(${(p.h / ROWS) * 100}% - ${gap}px)`,
  }
}

/** 曹操金色加大 / 兵小块区分色 / 将领骨白（墨金主题色） */
function pieceClass(p: Piece): string {
  if (p.id === 'cao')
    return 'hr-piece--cao'
  if (p.w === 1 && p.h === 1)
    return 'hr-piece--sol'
  return 'hr-piece--gen'
}

const ARROW_GLYPH: Record<Dir, string> = {
  up: '↑',
  down: '↓',
  left: '←',
  right: '→',
}

/** 箭头钉在选中块对应边中点（棋盘百分比，translate 居中跨边） */
function arrowStyle(dir: Dir): Record<string, string> {
  const p = selectedPiece.value
  if (!p)
    return {}
  const cx = ((p.x + p.w / 2) / COLS) * 100
  const cy = ((p.y + p.h / 2) / ROWS) * 100
  switch (dir) {
    case 'up':
      return { left: `${cx}%`, top: `${(p.y / ROWS) * 100}%` }
    case 'down':
      return { left: `${cx}%`, top: `${((p.y + p.h) / ROWS) * 100}%` }
    case 'left':
      return { left: `${(p.x / COLS) * 100}%`, top: `${cy}%` }
    case 'right':
      return { left: `${((p.x + p.w) / COLS) * 100}%`, top: `${cy}%` }
  }
}

/* ── 交互 ── */

/** 点块：通关锁定；再点同块取消，否则切换选中 */
function onPieceClick(p: Piece) {
  if (won.value)
    return
  selectedId.value = selectedId.value === p.id ? null : p.id
}

/** 点箭头：复校可动性（MP 视图层滞后防连点盲移）→ movePiece 重赋值 + 计步 + 判胜 */
function onArrow(dir: Dir) {
  const id = selectedId.value
  if (won.value || !id)
    return
  if (!canMove(pieces.value, id, dir))
    return
  pieces.value = movePiece(pieces.value, id, dir)
  steps.value++
  if (!isWin(pieces.value))
    return
  won.value = true
  selectedId.value = null
  const improved = best.value === null || steps.value < best.value
  if (improved) {
    best.value = steps.value
    uni.setStorageSync('huarong:best', steps.value)
    toast.success(`通关！新纪录 ${steps.value} 步`)
    return
  }
  toast.success(`通关！${steps.value} 步`)
}

/** 重置/重玩：回初始浅拷贝、步数清零、解锁 */
function onReset() {
  pieces.value = INITIAL_PIECES.map(p => ({ ...p }))
  steps.value = 0
  selectedId.value = null
  won.value = false
}
</script>

<template>
  <view
    class="hr-screen"
    :style="pageStyle"
  >
    <ModNav
      title="华容道"
      sub="趣味小工具 · 经典横刀立马"
      @back="onBack"
    />

    <view class="hr-content">
      <!-- 头部：步数 / 最佳 / 重置 -->
      <view class="hr-stats">
        <view class="hr-stat">
          <text class="hr-stat-k">
            步数
          </text>
          <text class="hr-stat-v">
            {{ steps }}
          </text>
        </view>
        <view class="hr-stat">
          <text class="hr-stat-k">
            最佳
          </text>
          <text class="hr-stat-v">
            {{ best ?? '—' }}
          </text>
        </view>
        <view
          class="hr-reset"
          hover-class="hr-pill--hover"
          @click="onReset"
        >
          <text class="hr-reset-t">
            重置
          </text>
        </view>
      </view>

      <!-- 棋盘：4×5 格底 + 10 枚绝对定位 + 方向箭头 + 通关锁定层 -->
      <view class="hr-board-wrap">
        <view
          class="hr-board"
          @click="selectedId = null"
        >
          <view
            class="hr-grid"
            :style="{
              gridTemplateColumns: `repeat(${COLS}, 1fr)`,
              gridTemplateRows: `repeat(${ROWS}, 1fr)`,
            }"
          >
            <view
              v-for="i in COLS * ROWS"
              :key="i"
              class="hr-cell"
            />
          </view>

          <view
            v-for="p in pieces"
            :key="p.id"
            class="hr-piece"
            :class="[pieceClass(p), { 'hr-piece--sel': selectedId === p.id }]"
            :style="pieceStyle(p)"
            hover-class="hr-piece--hover"
            @click.stop="onPieceClick(p)"
          >
            <text class="hr-piece-t">
              {{ p.name }}
            </text>
          </view>

          <!-- 可动方向箭头（不可动方向不渲染） -->
          <template v-if="!won && selectedId">
            <view
              v-for="dir in selDirs"
              :key="dir"
              class="hr-arrow"
              :style="arrowStyle(dir)"
              hover-class="hr-arrow--hover"
              @click.stop="onArrow(dir)"
            >
              <text class="hr-arrow-t">
                {{ ARROW_GLYPH[dir] }}
              </text>
            </view>
          </template>

          <!-- 通关锁定层 -->
          <view
            v-if="won"
            class="hr-win"
            @click.stop
          >
            <text class="hr-win-t">
              通关！{{ steps }} 步
            </text>
            <view
              class="hr-win-btn"
              hover-class="hr-pill--hover"
              @click.stop="onReset"
            >
              <text class="hr-win-btn-t">
                重玩
              </text>
            </view>
          </view>
        </view>
      </view>

      <view class="hr-priv">
        <view class="hr-priv-dot" />
        <text class="hr-priv-t">
          点选方块 · 箭头挪一步 · 最佳步数仅存本机
        </text>
      </view>
    </view>

    <view class="hr-footer">
      <text class="hr-footer-t">
        经典横刀立马 · 仅供娱乐参考
      </text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.hr-screen {
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
.hr-content {
  flex: 1;
  min-height: 0;
  padding: 6px 16px 12px;
  overflow-y: auto;
}

/* 宽屏收进 480 手机栏（box-sizing 显式，防窄屏溢出） */
.fcn-nav,
.hr-content,
.hr-footer {
  box-sizing: border-box;
  width: 100%;
  max-width: 480px;
  margin-inline: auto;
}

/* ── 头部统计行 ── */
.hr-stats {
  display: flex;
  gap: 10px;
  align-items: stretch;
  margin-bottom: 14px;
}

.hr-stat {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  align-items: center;
  justify-content: center;
  padding: 10px 8px;
  border-radius: 14px;
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.hr-stat-k {
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 12.5px;
  opacity: 0.7;
}

.hr-stat-v {
  font-family: 'Oswald', sans-serif;
  font-size: 24px;
  font-weight: 600;
  letter-spacing: 0.04em;
  color: var(--brass);
}

.hr-reset {
  display: flex;
  flex: none;
  align-items: center;
  justify-content: center;
  padding: 0 22px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--ink-700);
}

.hr-pill--hover {
  opacity: 0.82;
  transform: scale(0.97);
}

.hr-reset-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 500;
  letter-spacing: 0.05em;
}

/* ── 棋盘（外圈留白给跨边箭头） ── */
.hr-board-wrap {
  padding: 20px 18px;
}

.hr-board {
  position: relative;
  width: 100%;
  max-width: 360px;
  aspect-ratio: 4 / 5;
  margin: 0 auto;
  border: 1px solid var(--on-ink-a28);
  border-radius: var(--r-tile);
  background: var(--ink-700);
  box-shadow: var(--shadow-tile);
}

/* 4×5 格底（COLS/ROWS 内联注入） */
.hr-grid {
  position: absolute;
  inset: 0;
  display: grid;
  gap: 2px;
  padding: 2px;
}

.hr-cell {
  border-radius: 6px;
  background: var(--ink-900);
  opacity: 0.55;
}

/* ── 棋子（绝对定位，文案 = piece.name 数据驱动） ── */
.hr-piece {
  position: absolute;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  box-shadow: var(--shadow-tile);
  z-index: 1;
}

.hr-piece--hover {
  opacity: 0.85;
  transform: scale(0.97);
}

.hr-piece--sel {
  box-shadow: 0 0 0 2px var(--brass), var(--shadow-tile);
  z-index: 2;
}

/* 曹操：金色加大 */
.hr-piece--cao {
  background: var(--brass);
}

.hr-piece--cao .hr-piece-t {
  color: var(--ink-900);
  font-family: 'Noto Serif SC', serif;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

/* 将领：骨白 */
.hr-piece--gen {
  background: var(--tile);
}

.hr-piece--gen .hr-piece-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

/* 兵：小块区分色（墨金主题 jade） */
.hr-piece--sol {
  background: var(--jade);
}

.hr-piece--sol .hr-piece-t {
  color: var(--on-ink);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 13px;
  font-weight: 500;
}

.hr-piece-t {
  text-align: center;
  pointer-events: none;
}

/* ── 方向箭头（跨边居中，只渲染可动方向） ── */
.hr-arrow {
  position: absolute;
  display: flex;
  width: 32px;
  height: 32px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--brass-a50);
  border-radius: 50%;
  background: var(--ink-900);
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.4);
  transform: translate(-50%, -50%);
  z-index: 3;
}

/* hover 须保留 translate 居中（共享 hr-pill--hover 的 scale 会顶掉定位 transform） */
.hr-arrow--hover {
  opacity: 0.82;
  transform: translate(-50%, -50%) scale(0.97);
}

.hr-arrow-t {
  color: var(--brass);
  font-family: 'Oswald', sans-serif;
  font-size: 18px;
  font-weight: 600;
  line-height: 1;
}

/* ── 通关锁定层 ── */
.hr-win {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
  align-items: center;
  justify-content: center;
  border-radius: var(--r-tile);
  background: rgba(20, 18, 15, 0.82);
  z-index: 4;
}

.hr-win-t {
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 24px;
  font-weight: 700;
  letter-spacing: 0.06em;
}

.hr-win-btn {
  display: flex;
  height: 44px;
  align-items: center;
  justify-content: center;
  padding: 0 30px;
  border-radius: 999px;
  background: var(--brass);
}

.hr-win-btn-t {
  color: var(--ink-900);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 700;
  letter-spacing: 0.05em;
}

/* ── 本机提示 ── */
.hr-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  justify-content: center;
  margin-top: 4px;
}

.hr-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade);
}

.hr-priv-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}

/* ── 底注（钉底） ── */
.hr-footer {
  flex: none;
  padding: 10px 16px 8px;
  text-align: center;
}

.hr-footer-t {
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
}
</style>
