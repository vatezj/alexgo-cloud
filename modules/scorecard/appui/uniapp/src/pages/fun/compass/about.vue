<script setup lang="ts">
/**
 * 关于与免责 pages/fun/compass/about（设计 preview/index-about.html 屏 02；spec 合规文案）
 *
 * - 免责全文四段 copy.ts 逐字；「清除本工具全部档案」= 朱红描边危险钮
 * - 清除走 useDialog('globalDialog') 确认 → store.clearAll()（只清 fun_compass_archives）
 *   → toast CLEAR_DONE_TOAST（账本数据不受影响）
 * - 底部仍带两句免责（spec：六页齐全）
 */
import ModDisclaimer from '@/components/funCompass/ModDisclaimer.vue'
import ModNav from '@/components/funCompass/ModNav.vue'
import { useSystemBar } from '@/composables/useSystemBar'
import { useFunCompassArchivesStore } from '@/store/funCompass/archives'
import {
  ABOUT_PARAGRAPHS,
  ABOUT_TITLE,
  CLEAR_BUTTON,
  CLEAR_DONE_TOAST,
  CLEAR_HINT,
} from '@/utils/funCompass/copy'

definePage({
  style: {
    navigationStyle: 'custom',
  },
})

const router = useRouter()
const toast = useToast()
// 裸 useDialog() inject 不到该 key 会自建孤儿 ref，confirm 永不弹且 promise 挂起。
const dialog = useDialog('globalDialog')
const { pageStyle } = useSystemBar()
const store = useFunCompassArchivesStore()

const clearing = ref(false)

async function onClear() {
  if (clearing.value)
    return
  clearing.value = true
  try {
    try {
      await dialog.confirm({
        title: '清除本工具全部档案',
        msg: '只清空趣味方位工具的出生档案，不会影响您的账本记录，确认清除？',
        confirmButtonText: '确认清除',
        cancelButtonText: '取消',
      })
    }
    catch {
      return
    }
    store.clearAll()
    toast.success({ msg: CLEAR_DONE_TOAST })
  }
  finally {
    clearing.value = false
  }
}

function goBack() {
  if (getCurrentPages().length > 1)
    uni.navigateBack()
  else
    router.replaceAll({ path: '/pages/fun/compass/index' })
}
</script>

<template>
  <view
    class="fc-page"
    :style="pageStyle"
  >
    <ModNav
      title="关于与免责"
      @back="goBack"
    />

    <!-- 免责全文卡（四段逐字） -->
    <view class="fc-about-card">
      <view class="fc-about-head">
        <text class="fc-about-t">
          {{ ABOUT_TITLE }}
        </text>
        <text class="fc-about-seal">
          免责
        </text>
      </view>

      <text
        v-for="(p, i) in ABOUT_PARAGRAPHS"
        :key="i"
        class="fc-about-p"
      >
        {{ p }}
      </text>

      <view class="fc-priv">
        <view class="fc-priv-dot" />
        <text class="fc-priv-t">
          出生信息仅存本机，不上传
        </text>
      </view>
    </view>

    <!-- 危险操作：只清本模块档案 -->
    <view
      class="fc-danger"
      @click="onClear"
    >
      {{ CLEAR_BUTTON }}
    </view>
    <text class="fc-danger-hint">
      {{ CLEAR_HINT }}
    </text>

    <view class="fc-spacer" />

    <ModDisclaimer />
  </view>
</template>

<style lang="scss" scoped>
.fc-page {
  display: flex;
  box-sizing: border-box;
  flex-direction: column;
  min-height: 100vh;
  padding: calc(var(--status-bar-h, env(safe-area-inset-top, 0px)) + 6px) 16px calc(env(safe-area-inset-bottom, 0px) + 16px);
  background: var(--ink-900);
  background-image: var(--spot);
  color: var(--on-ink);
}

.fc-about-card {
  margin-bottom: 16px;
  padding: 18px 16px;
  border-radius: var(--r-tile);
  background: var(--tile);
  box-shadow: var(--shadow-tile);
  color: var(--jet);
}

.fc-about-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;
}

.fc-about-t {
  color: var(--jet);
  font-family: 'Noto Serif SC', serif;
  font-size: 23px;
  font-weight: 700;
  letter-spacing: 0.04em;
}

.fc-about-seal {
  padding: 3px 10px;
  border-radius: 8px;
  background: var(--vermillion);
  color: var(--tile);
  font-family: 'Noto Serif SC', serif;
  font-size: 13px;
  font-weight: 700;
}

.fc-about-p {
  display: block;
  margin-bottom: 12px;
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 14px;
  line-height: 1.75;
}

.fc-priv {
  display: flex;
  gap: 6px;
  align-items: center;
  margin-top: 4px;
}

.fc-priv-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--jade);
}

.fc-priv-t {
  color: var(--jet);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  opacity: 0.65;
}

/* 朱红描边危险钮（非实心，触达 48） */
.fc-danger {
  display: flex;
  height: 48px;
  align-items: center;
  justify-content: center;
  border: 1.5px solid var(--vermillion);
  border-radius: 999px;
  color: var(--vermillion-hi, var(--vermillion));
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 15.5px;
  font-weight: 600;
}

.fc-danger-hint {
  display: block;
  margin-top: 8px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 11.5px;
  text-align: center;
}

.fc-spacer {
  flex: 1;
  min-height: 16px;
}
</style>
