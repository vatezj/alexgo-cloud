<script setup lang="ts">
/**
 * 邀请弹层（设计 03：邀请 · 扇牌小程序码）
 *
 * - 扇牌码：PlayCard（A♥ 骨白牌）内嵌 getRoomWxacode 的 PNG dataURL；
 *   打开时惰性拉取并按房间 id 缓存（同房间重开不重拉，换房间重拉）
 * - 失败降级（接口 throw / 空缓冲 / 非 PNG 字节）：牌面内改示房号大字 + 复制房号，不崩不回环
 * - 转发给好友：MP 微信 button open-type="share"（触发房间页 onShareAppMessage，path 带 roomId）；
 *   H5 走复制房号（与降级复制同一动作，uni.setClipboardData）
 * - 文案：「邀请好友扫描牌背小程序码加入房间」（逐字 brief）；
 *   本地业务错误 toast（ApiError 非 401/403，handlers 已全局提示的不重复）
 * - 色值零硬编码：全走主题 token（--tile/--jet/--brass/--dim/--on-ink 等）
 */
import { ApiError } from '@/api/core/handlers'
import { getRoomWxacode } from '@/api/scorecard/wxacode'
import PlayCard from '@/components/scorecard/PlayCard.vue'

interface Props {
  show: boolean
  /** 房间 id（小程序码请求参数） */
  roomId: number
  /** 房号原文（大字展示 + 复制兜底；快照 room_code） */
  roomCode: string
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
}>()

const toast = useToast()

/** 小程序码 dataURL 缓存（按房间 id：同房间重开复用，换房间失效重拉）。 */
const cache = ref<{ roomId: number, dataUrl: string } | null>(null)
const dataUrl = ref('')
const loading = ref(false)

/** 业务信封错误本层提示；401/403（handlers 已清态全局提示）不重复 toast。 */
function toastApiError(err: unknown) {
  if (err instanceof ApiError && err.code !== 401 && err.code !== 403)
    toast.error({ msg: err.message || '请求失败' })
}

/** PNG 魔数校验：空/过短/非 PNG 字节按失败降级（不缓存，下次打开可重试）。 */
function isPng(buf: ArrayBuffer): boolean {
  if (!buf || buf.byteLength < 8)
    return false
  const b = new Uint8Array(buf)
  return b[0] === 0x89 && b[1] === 0x50 && b[2] === 0x4E && b[3] === 0x47
}

/** ArrayBuffer → base64 dataURL（分块转义，避免大缓冲直接展开爆栈）。 */
function toDataUrl(buf: ArrayBuffer): string {
  const bytes = new Uint8Array(buf)
  let bin = ''
  const chunk = 0x8000
  for (let i = 0; i < bytes.length; i += chunk)
    bin += String.fromCharCode(...Array.from(bytes.subarray(i, i + chunk)))
  return `data:image/png;base64,${btoa(bin)}`
}

/** 打开时取码：有缓存直接用；无缓存/换房间才请求。失败保持 dataUrl 空 → 牌面降级。 */
async function loadWxacode() {
  const roomId = props.roomId
  if (!roomId || loading.value)
    return
  if (cache.value && cache.value.roomId === roomId) {
    dataUrl.value = cache.value.dataUrl
    return
  }
  loading.value = true
  dataUrl.value = ''
  try {
    const buf = await getRoomWxacode(roomId)
    if (!isPng(buf))
      return
    const url = toDataUrl(buf)
    cache.value = { roomId, dataUrl: url }
    dataUrl.value = url
  }
  catch (err) {
    toastApiError(err)
  }
  finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (show) => {
    if (show)
      void loadWxacode()
  },
  { immediate: true },
)

/** 复制房号（H5 转发动作 + 牌面降级兜底共用）。成功提示由端上剪贴板组件自带。 */
function copyRoomCode() {
  if (!props.roomCode) {
    toast.warning('房号未就绪')
    return
  }
  uni.setClipboardData({
    data: props.roomCode,
    fail: () => toast.error('复制失败，请重试'),
  })
}
</script>

<template>
  <wd-popup
    :model-value="show"
    position="center"
    closable
    custom-class="qr-invite-pop"
    custom-style="background: transparent;"
    @update:model-value="emit('update:show', $event)"
  >
    <view class="qr-invite">
      <!-- 房号大字 -->
      <view class="head">
        <text class="head-label">
          房号
        </text>
        <text class="head-no num">
          {{ roomCode || '····' }}
        </text>
      </view>

      <!-- 扇牌小程序码（幽灵牌 + A♥ 牌面） -->
      <view class="qr-stack">
        <view class="ghost g1" />
        <view class="ghost g2" />
        <PlayCard
          class="qr-card"
          suit="♥"
          rank="A"
        >
          <view class="qr-body">
            <image
              v-if="dataUrl"
              class="qr-img"
              :src="dataUrl"
              mode="aspectFit"
            />
            <view
              v-else
              class="qr-fallback"
            >
              <text class="fb-cap">
                {{ loading ? '小程序码生成中…' : '小程序码没出来 · 报房号也行' }}
              </text>
              <text class="fb-code num">
                {{ roomCode || '····' }}
              </text>
              <button
                class="fb-btn"
                @click="copyRoomCode"
              >
                复制房号
              </button>
            </view>
            <text class="qr-cap">
              扫码加入 · 或点下方转发
            </text>
          </view>
        </PlayCard>
      </view>

      <text class="invite-sub">
        邀请好友扫描牌背小程序码加入房间
      </text>

      <!-- #ifdef MP-WEIXIN -->
      <button
        class="btn brass invite-cta"
        open-type="share"
      >
        转发给好友
      </button>
      <!-- #endif -->
      <!-- #ifndef MP-WEIXIN -->
      <button
        class="btn brass invite-cta"
        @click="copyRoomCode"
      >
        转发给好友
      </button>
      <!-- #endif -->
    </view>
  </wd-popup>
</template>

<style lang="scss" scoped>
/* 弹层根：牌桌底透出（背景遮罩走 wd-popup 默认），关闭钮走骨白 */
.qr-invite-pop {
  --wot-popup-bg: transparent;
  --wot-popup-radius: 0;
  --wot-popup-close-color: var(--on-ink);
}

.qr-invite {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 10px 28px calc(env(safe-area-inset-bottom, 0px) + 24px);
}

/* ── 房号大字 ── */
.head {
  text-align: center;
}

.head-label {
  display: block;
  margin-bottom: 6px;
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 0.3em;
}

.head-no {
  color: var(--brass);
  font-size: 56px;
  line-height: 1;
  letter-spacing: 0.05em;
}

/* ── 扇牌码 ── */
.qr-stack {
  position: relative;
  width: 230px;
  height: 250px;
  margin-top: 22px;
}

.qr-stack .ghost {
  position: absolute;
  top: 14px;
  left: 20px;
  width: 190px;
  height: 230px;
  border-radius: 14px;
  box-shadow: 0 6px 18px var(--ink-900);
}

.qr-stack .g1 {
  background: var(--tile-shade);
  transform: rotate(-9deg);
}

.qr-stack .g2 {
  background: var(--tile);
  transform: rotate(7deg);
}

.qr-card {
  position: absolute;
  top: 6px;
  left: 17px;
  width: 196px;
  height: 238px;
}

.qr-body {
  display: flex;
  height: 100%;
  flex-direction: column;
  align-items: center;
  padding: 26px 14px 12px;
}

.qr-img {
  width: 168px;
  height: 168px;
  border: 1px solid var(--tile-emboss);
  border-radius: 8px;
  background: var(--tile);
}

/* 降级：没码 → 牌面内房号大字 + 复制按钮 */
.qr-fallback {
  display: flex;
  width: 168px;
  height: 168px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  box-sizing: border-box;
  padding: 12px;
  border: 1px dashed var(--tile-emboss);
  border-radius: 8px;
}

.fb-cap {
  color: var(--tile-emboss);
  font-size: 10.5px;
  line-height: 1.5;
  text-align: center;
}

.fb-code {
  color: var(--jet);
  font-size: 34px;
  line-height: 1;
  letter-spacing: 0.08em;
}

.fb-btn {
  display: inline-flex;
  height: 34px;
  align-items: center;
  justify-content: center;
  padding: 0 18px;
  border: none;
  border-radius: 999px;
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  box-shadow: 0 3px 0 var(--brass-lo);
  color: var(--ink-900);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
}

.fb-btn::after {
  border: none;
}

.qr-cap {
  margin-top: 10px;
  color: var(--tile-emboss);
  font-size: 11px;
  text-align: center;
}

/* ── 文案 + CTA ── */
.invite-sub {
  margin-top: 20px;
  color: var(--dim);
  font-size: 12.5px;
}

.invite-cta {
  width: 280px;
  margin-top: 16px;
}
</style>
