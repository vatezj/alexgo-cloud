<script setup lang="ts">
/**
 * 语音 5 档循环开关（设计 10 · 头像弹窗；手册 §3.8：点头像弹窗喇叭 5 档循环）
 *
 * - 展示当前档（图标 + 文案 + 副文案），点按 → nextMode 循环下一档
 *   （all-off → all-on → entertain-off → ledger-off → ops-only → all-off）
 * - 档位态在 voiceStore（persist $id='scorecard-voice'）
 * - 色值零硬编码：全走主题 token（--on-ink/--dim/--brass/--on-ink-a04/--on-ink-a28）
 */
import {
  modeFromFlags,
  useScorecardVoiceStore,
  VOICE_MODE_DESCS,
  VOICE_MODE_ICONS,
  VOICE_MODE_LABELS,
} from '@/store/scorecard/voice'

const voiceStore = useScorecardVoiceStore()

/** 当前档（三 bool 反查；坏数据回退 ops-only）。 */
const currentMode = computed(() => modeFromFlags(voiceStore))
const icon = computed(() => VOICE_MODE_ICONS[currentMode.value])
const label = computed(() => VOICE_MODE_LABELS[currentMode.value])
const desc = computed(() => VOICE_MODE_DESCS[currentMode.value])

/** 点按循环下一档（store.cycle 为唯一入口）。 */
function onTap() {
  voiceStore.cycle()
}
</script>

<template>
  <view
    class="voice-toggle"
    @click="onTap"
  >
    <view class="vt-label">
      <wd-icon
        css-icon
        class="vt-label-icon"
        name="i-carbon-volume-down"
      />
      语音播放
    </view>
    <view class="vt-right">
      <view class="vt-chip">
        <wd-icon
          css-icon
          class="vt-icon"
          :name="icon"
        />
        <text class="vt-text">
          {{ label }}
        </text>
      </view>
      <text class="vt-cap">
        {{ desc || '5 档循环 · 点按切换' }}
      </text>
    </view>
  </view>
</template>

<style lang="scss" scoped>
.voice-toggle {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid var(--on-ink-a28);
  border-radius: 12px;
  background: var(--on-ink-a04);
  cursor: pointer;
}

.vt-label {
  display: flex;
  flex: none;
  align-items: center;
  gap: 4px;
  color: var(--on-ink);
  font-size: 14px;
}

.vt-label-icon {
  font-size: 14px;
  line-height: 1;
}

.vt-right {
  display: flex;
  min-width: 0;
  flex-direction: column;
  align-items: flex-end;
  gap: 2px;
}

.vt-chip {
  display: flex;
  align-items: center;
  gap: 5px;
  padding: 4px 10px;
  border: 1px solid var(--brass-a50);
  border-radius: 999px;
  background: var(--on-ink-a04);
}

.vt-icon {
  font-size: 13px;
  line-height: 1;
}

.vt-text {
  color: var(--brass);
  font-size: 12.5px;
}

.vt-cap {
  color: var(--dim);
  font-size: 10px;
}
</style>
