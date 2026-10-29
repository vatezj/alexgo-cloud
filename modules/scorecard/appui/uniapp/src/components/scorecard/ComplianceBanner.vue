<script lang="ts">
import { ref } from 'vue'

/**
 * 本会话关闭记忆（模块级单例，键名保留 storage 语义）：
 * 小程序无 sessionStorage；setStorageSync 会跨会话持久，故用模块内存承载「本会话」语义。
 */
const DISMISS_KEY = 'scorecard-compliance-dismissed'
const dismissedMap = ref<Record<string, boolean>>({})
</script>

<script setup lang="ts">
/**
 * 合规横幅（设计 02 comply；锁定红，四套主题同值，走全局 .comply 工具类）
 *
 * - 可关：本会话记住（多页面共享同一记忆，关一次本会话不再出现）
 * - 「了解详情」→ 使用手册页；文案为产品锁定合规语，逐字保留
 */
const dismissed = computed(() => Boolean(dismissedMap.value[DISMISS_KEY]))

const router = useRouter()

function dismiss() {
  dismissedMap.value[DISMISS_KEY] = true
}

function goManual() {
  router.push({ path: '/pages/manual/index' })
}
</script>

<template>
  <view
    v-if="!dismissed"
    class="comply"
  >
    <wd-icon
      css-icon
      class="spk"
      name="i-carbon-volume-up"
    />
    <text class="txt">
      “打牌记账”<b>不涉及金钱，禁止用于违法行为</b>
    </text>
    <text
      class="link"
      @click="goManual"
    >
      了解详情
    </text>
    <wd-icon
      css-icon
      class="x"
      name="i-carbon-close"
      @click="dismiss"
    />
  </view>
</template>

<style lang="scss" scoped>
.comply .txt {
  min-width: 0;
  flex: 1;
}

.comply .txt b {
  color: var(--comply-strong);
  font-weight: 700;
}

.comply .link {
  flex: none;
  margin-left: 4px;
  color: var(--comply-strong);
  font-size: 12px;
  font-weight: 500;
  text-decoration: underline;
}
</style>
