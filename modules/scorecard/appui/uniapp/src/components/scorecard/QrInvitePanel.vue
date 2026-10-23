<script setup lang="ts">
/**
 * 邀请弹层（设计 03：邀请 · 扇牌小程序码）
 *
 * - 主次：小程序码为第一视觉（牌面主体，打开即拉）；房号退居牌下小胶囊
 *   （码就绪时展示，点按复制）——扫码是主路径，报房号是辅路径
 * - 扇牌码：PlayCard（A♥ 骨白牌）内嵌 getRoomWxacode 的 PNG dataURL；
 *   打开时惰性拉取并按房间 id 缓存（同房间重开不重拉，换房间重拉）
 * - 失败降级（接口 throw / 空缓冲 / 非 PNG 字节）：牌面内改示房号大字 + 复制房号，
 *   牌下胶囊同步隐藏（牌面已是房号主场，不重复），不崩不回环
 * - 转发给好友：MP 微信 button open-type="share"（触发房间页 onShareAppMessage，path 走 scene 进房协议）；
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

/** base64 字母表：btoa 是浏览器 Window API，MP-WEIXIN 逻辑层没有，跨端须手写编码。 */
const B64_CHARS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/'

/** 字节流 → base64（纯 JS 逐三字节编码，不依赖 btoa / wx.arrayBufferToBase64，H5/小程序/App 一致）。 */
function toBase64(bytes: Uint8Array): string {
  let out = ''
  for (let i = 0; i < bytes.length; i += 3) {
    const b0 = bytes[i]
    const has1 = i + 1 < bytes.length
    const has2 = i + 2 < bytes.length
    const b1 = has1 ? bytes[i + 1] : 0
    const b2 = has2 ? bytes[i + 2] : 0
    out += B64_CHARS[b0 >> 2]
    out += B64_CHARS[((b0 & 0x03) << 4) | (b1 >> 4)]
    out += has1 ? B64_CHARS[((b1 & 0x0F) << 2) | (b2 >> 6)] : '='
    out += has2 ? B64_CHARS[b2 & 0x3F] : '='
  }
  return out
}

/** ArrayBuffer → base64 dataURL（逐字节直编，无大字符串展开、无平台 API 依赖）。 */
function toDataUrl(buf: ArrayBuffer): string {
  return `data:image/png;base64,${toBase64(new Uint8Array(buf))}`
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
      <!-- 扇牌小程序码（主视觉：幽灵牌 + A♥ 牌面） -->
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

      <!-- 房号辅行（码就绪时展示，点按复制；降级态牌面已是房号主场则隐藏） -->
      <view
        v-if="dataUrl && roomCode"
        class="code-pill"
        @click="copyRoomCode"
      >
        <text class="code-label">
          房号
        </text>
        <text class="code-no num">
          {{ roomCode }}
        </text>
        <text class="code-copy">
          复制
        </text>
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
  padding: 22px 28px calc(env(safe-area-inset-bottom, 0px) + 24px);
}

/* ── 扇牌码（主视觉） ── */
.qr-stack {
  position: relative;
  width: 230px;
  height: 250px;
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
  flex-direction: column;
  align-items: center;
  height: 100%;
  padding: 26px 14px 12px;
}

.qr-img {
  width: 168px;
  height: 168px;
  background: var(--tile);
  border: 1px solid var(--tile-emboss);
  border-radius: 8px;
}

/* 降级：没码 → 牌面内房号大字 + 复制按钮 */
.qr-fallback {
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  gap: 8px;
  align-items: center;
  justify-content: center;
  width: 168px;
  height: 168px;
  padding: 12px;
  border: 1px dashed var(--tile-emboss);
  border-radius: 8px;
}

.fb-cap {
  font-size: 10.5px;
  line-height: 1.5;
  color: var(--tile-emboss);
  text-align: center;
}

.fb-code {
  font-size: 34px;
  line-height: 1;
  color: var(--jet);
  letter-spacing: 0.08em;
}

.fb-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 34px;
  padding: 0 18px;
  font-size: 13px;
  font-weight: 500;
  line-height: 1;
  color: var(--ink-900);
  background: linear-gradient(180deg, var(--brass-hi), var(--brass));
  border: none;
  border-radius: 999px;
  box-shadow: 0 3px 0 var(--brass-lo);
}

.fb-btn::after {
  border: none;
}

.qr-cap {
  margin-top: 10px;
  font-size: 11px;
  color: var(--tile-emboss);
  text-align: center;
}

/* ── 房号辅行胶囊（点按复制） ── */
.code-pill {
  display: flex;
  gap: 8px;
  align-items: center;
  padding: 7px 16px;
  margin-top: 18px;
  background: var(--on-ink-a04);
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
}

.code-label {
  font-size: 12px;
  color: var(--dim);
  letter-spacing: 0.2em;
}

.code-no {
  font-size: 17px;
  line-height: 1;
  color: var(--brass);
  letter-spacing: 0.08em;
}

.code-copy {
  font-size: 11px;
  color: var(--on-ink);
  opacity: 0.75;
}

/* ── 文案 + CTA ── */
.invite-sub {
  margin-top: 18px;
  font-size: 12.5px;
  color: var(--dim);
}

.invite-cta {
  width: 280px;
  margin-top: 16px;
}
</style>
