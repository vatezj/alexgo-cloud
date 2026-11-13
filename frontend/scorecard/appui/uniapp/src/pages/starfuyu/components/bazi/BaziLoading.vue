<!--
  排盘/求签全屏加载遮罩（迁自 luckyU bazhi/components/BaziLoading.vue）
  - 纯 CSS 太极动画 + 轮播提示；无 wot-ui、无 API → 模板/样式原样保留
  - ref/onMounted/onUnmounted 走 appui 自动导入
-->
<script setup lang="ts">
interface Props {
  type?: 'bazi' | 'divination'
  message?: string
}

const props = withDefaults(defineProps<Props>(), {
  type: 'bazi',
  message: '',
})

// 动态提示文案
const tips = ref<string[]>([])
const currentTip = ref('')
let tipIndex = 0

// 根据类型设置不同的提示文案
const tipMessages = {
  bazi: [
    '正在排盘测算...',
    '分析生辰八字...',
    '计算五行属性...',
    '生成命盘结果...',
  ],
  divination: [
    '正在摇签中...',
    '解读签文内容...',
    '分析运势走向...',
    '生成解签结果...',
  ],
}

// 轮播提示文案
function startTipRotation() {
  const messages = tipMessages[props.type]
  tips.value = props.message ? [props.message, ...messages] : messages

  const rotateTip = () => {
    currentTip.value = tips.value[tipIndex % tips.value.length]
    tipIndex++
  }

  rotateTip()
  const timer = setInterval(rotateTip, 2000)

  onUnmounted(() => {
    clearInterval(timer)
  })
}

onMounted(() => {
  startTipRotation()
})
</script>

<template>
  <view class="bazi-loading">
    <view class="loading-content">
      <!-- 太极图动画 -->
      <view class="yin-yang-loading">
        <view class="yin-yang">
          <view class="yin-yang-light">
            <view class="yin-yang-light-dot" />
          </view>
          <view class="yin-yang-dark">
            <view class="yin-yang-dark-dot" />
          </view>
        </view>
      </view>

      <!-- 提示文案 -->
      <view class="loading-text">
        <text class="loading-message">
          {{ currentTip }}
        </text>
        <view class="loading-dots">
          <text class="dot dot-1">
            .
          </text>
          <text class="dot dot-2">
            .
          </text>
          <text class="dot dot-3">
            .
          </text>
        </view>
      </view>

      <!-- 副提示 -->
      <text class="loading-subtitle">
        请稍候，正在为您精心测算
      </text>
    </view>
  </view>
</template>

<style scoped lang="scss">
.bazi-loading {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(243, 229, 216, 0.95) 0%, rgba(255, 255, 255, 0.98) 100%);
  backdrop-filter: blur(10rpx);
  z-index: 9999;
}

.loading-content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 40rpx;
  padding: 60rpx;
}

.yin-yang-loading {
  display: flex;
  justify-content: center;
  margin-bottom: 20rpx;
}

.yin-yang {
  width: 120rpx;
  height: 120rpx;
  border-radius: 50%;
  background: linear-gradient(to right, #8b0000 50%, #d4af37 50%);
  position: relative;
  overflow: hidden;
  animation: spin 3s linear infinite;
  border: 3rpx solid #8b0000;
  box-shadow: 0 0 30rpx rgba(139, 0, 0, 0.3);
}

.yin-yang-light {
  position: absolute;
  top: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 60rpx;
  height: 60rpx;
  border-radius: 50%;
  background-color: #d4af37;
}

.yin-yang-light-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 18rpx;
  height: 18rpx;
  border-radius: 50%;
  background-color: #8b0000;
}

.yin-yang-dark {
  position: absolute;
  bottom: 0;
  left: 50%;
  transform: translateX(-50%);
  width: 60rpx;
  height: 60rpx;
  border-radius: 50%;
  background-color: #8b0000;
}

.yin-yang-dark-dot {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 18rpx;
  height: 18rpx;
  border-radius: 50%;
  background-color: #d4af37;
}

@keyframes spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

.loading-text {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8rpx;
}

.loading-message {
  font-family: 'ZhiMangXing', cursive;
  font-size: 36rpx;
  color: #8b0000;
  letter-spacing: 4rpx;
}

.loading-dots {
  display: flex;
  gap: 4rpx;
}

.dot {
  font-family: 'ZhiMangXing', cursive;
  font-size: 36rpx;
  color: #8b0000;
  animation: blink 1.5s infinite;
}

.dot-1 {
  animation-delay: 0s;
}

.dot-2 {
  animation-delay: 0.3s;
}

.dot-3 {
  animation-delay: 0.6s;
}

@keyframes blink {
  0%, 100% {
    opacity: 0.3;
  }
  50% {
    opacity: 1;
  }
}

.loading-subtitle {
  font-family: 'NotoSerifSC', serif;
  font-size: 24rpx;
  color: #666666;
  letter-spacing: 2rpx;
  margin-top: 20rpx;
}
</style>
