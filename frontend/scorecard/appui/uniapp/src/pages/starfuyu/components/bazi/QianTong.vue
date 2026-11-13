<!--
  签筒 + 签文结果（迁自 luckyU bazhi/components/QianTong.vue）
  - 类型换 Task 1 `Divination`（@/api/starfuyu，字段同名：sign_number/sign_content/
    interpretation/lucky_direction/lucky_color/advice 可空列 → 模板插值空串兜底）
  - 签文解析 parseSignContent 与样式原样保留；无 wot-ui 控件
-->
<script setup lang="ts">
import type { Divination } from '@/api/starfuyu'

defineProps<{
  showResult: boolean
  result?: Divination
}>()

const emit = defineEmits<{
  shake: []
}>()

function handleShake() {
  emit('shake')
}

// 解析签文内容
function parseSignContent(signContent: string) {
  const lines = signContent.split('\n').filter(line => line.trim())
  const numberMatch = signContent.match(/第[一二三四五六七八九十百千万\d]+签/)
  const levelMatch = signContent.match(/(上上|上中|上下|中上|中中|中下|下上|下中|下下|上吉|中吉|下吉|大吉|小吉|平|凶|大凶)/)

  const verses: string[] = []
  const poem: string[] = []
  let isPoem = false

  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed)
      continue

    // 跳过签号和等级行
    if (numberMatch && trimmed.includes(numberMatch[0]))
      continue
    if (levelMatch && trimmed.includes(levelMatch[0]))
      continue

    // 判断是否是解曰部分
    if (trimmed.includes('解曰：') || trimmed.startsWith('解曰')) {
      isPoem = true
      continue
    }

    if (isPoem) {
      poem.push(trimmed)
    }
    else {
      // 诗句通常是4-7个字
      if (trimmed.length >= 4 && trimmed.length <= 14 && !trimmed.includes('：')) {
        verses.push(trimmed)
      }
    }
  }

  return {
    number: numberMatch ? numberMatch[0] : '',
    level: levelMatch ? levelMatch[0] : '',
    verses: verses.length > 0 ? verses : [],
    poem: poem.join('\n'),
  }
}
</script>

<template>
  <view class="qiantong-area">
    <view class="qiantong-img" @click="handleShake">
      <view class="stick stick-1" />
      <view class="stick stick-2" />
      <view class="stick stick-3" />
      <view class="stick-label-wrap">
        <text class="stick-label-text">
          求签
        </text>
      </view>
    </view>
  </view>

  <!-- 签文结果 -->
  <view v-if="showResult && result" class="result-scroll">
    <text class="result-title">
      【 {{ parseSignContent(result.sign_content).number || `第 ${result.sign_number} 签` }} 】
    </text>
    <text class="result-sub">
      {{ parseSignContent(result.sign_content).level || '上吉' }}
    </text>
    <view class="result-verses">
      <view
        v-for="(verse, idx) in parseSignContent(result.sign_content).verses"
        :key="idx"
        class="result-verse"
      >
        <text class="verse-text">
          {{ verse }}
        </text>
      </view>
    </view>
    <view v-if="parseSignContent(result.sign_content).poem" class="poem-section">
      <text class="poem-label">
        解曰：
      </text>
      <text class="poem">
        {{ parseSignContent(result.sign_content).poem }}
      </text>
    </view>
    <view class="result-details">
      <view class="detail-item">
        <text class="detail-label">
          问题：
        </text>
        <text class="detail-value">
          {{ result.question }}
        </text>
      </view>
      <view class="detail-item">
        <text class="detail-label">
          分类：
        </text>
        <text class="detail-value">
          {{ result.category }}
        </text>
      </view>
      <view class="detail-item">
        <text class="detail-label">
          幸运方向：
        </text>
        <text class="detail-value">
          {{ result.lucky_direction }}
        </text>
      </view>
      <view class="detail-item">
        <text class="detail-label">
          幸运颜色：
        </text>
        <text class="detail-value">
          {{ result.lucky_color }}
        </text>
      </view>
      <view class="interpretation-section">
        <text class="interpretation-label">
          解签：
        </text>
        <text class="interpretation-text">
          {{ result.interpretation }}
        </text>
      </view>
      <view class="advice-section">
        <text class="advice-label">
          建议：
        </text>
        <text class="advice-text">
          {{ result.advice }}
        </text>
      </view>
    </view>
  </view>
</template>

<style scoped lang="scss">
/* ========== 签筒区域 ========== */
.qiantong-area {
  display: flex;
  justify-content: center;
  padding: 20rpx 0;
}
.qiantong-img {
  width: 240rpx;
  height: 320rpx;
  background: linear-gradient(to right, #654321, #8b4513, #654321);
  border-radius: 20rpx 20rpx 80rpx 80rpx;
  border: 4rpx solid #3e2723;
  box-shadow: 0 20rpx 40rpx rgba(0, 0, 0, 0.2);
  position: relative;
  display: flex;
  justify-content: center;
  overflow: hidden;
}

/* 签棍 */
.stick {
  width: 24rpx;
  background-color: #d2b48c;
  border: 2rpx solid #8b4513;
  position: absolute;
  bottom: 40rpx;
  border-radius: 4rpx;
}
.stick-1 {
  height: 280rpx;
  transform: rotate(-15deg) translate(-40rpx, 20rpx);
}
.stick-2 {
  height: 300rpx;
  transform: rotate(-5deg) translate(-10rpx, 0rpx);
}
.stick-3 {
  height: 280rpx;
  transform: rotate(10deg) translate(30rpx, 10rpx);
}

/* 求签按钮 */
.stick-label-wrap {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 120rpx;
  height: 120rpx;
  background-color: #8b0000;
  border-radius: 50%;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 4rpx solid #d4af37;
  z-index: 5;
}
.stick-label-text {
  font-family: 'ZhiMangXing', cursive;
  font-size: 48rpx;
  color: #d4af37;
}

/* ========== 签文结果 ========== */
.result-scroll {
  margin-top: 30rpx;
  background-color: #ffffff;
  padding: 40rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  border-top: 8rpx solid #8b0000;
  border-bottom: 8rpx solid #8b0000;
}
.result-title {
  font-family: 'ZhiMangXing', cursive;
  font-size: 48rpx;
  color: #8b0000;
  margin-bottom: 20rpx;
}
.result-sub {
  font-family: 'ZhiMangXing', cursive;
  font-size: 32rpx;
  color: #8b0000;
  margin-bottom: 40rpx;
}
.result-verses {
  display: flex;
  flex-direction: row;
  justify-content: center;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 30rpx 20rpx;
  width: 100%;
  max-width: 100%;
  padding: 0 20rpx;
}
.result-verse {
  flex: 0 0 auto;
  min-width: 0;
  max-width: calc(50% - 10rpx);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 10rpx;
}
.verse-text {
  font-family: 'LongCang', cursive;
  font-size: 36rpx;
  line-height: 1.8;
  color: #333333;
  letter-spacing: 2rpx;
  word-wrap: break-word;
  word-break: break-all;
  white-space: normal;
  text-align: center;
  display: block;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}
.poem-section {
  margin-top: 30rpx;
  width: 100%;
}

.poem-label {
  font-family: 'ZhiMangXing', cursive;
  font-size: 32rpx;
  color: #8b0000;
  margin-bottom: 10rpx;
  display: block;
}

.poem {
  font-size: 28rpx;
  color: #666666;
  font-style: italic;
  font-family: 'NotoSerifSC', serif;
  line-height: 1.8;
  display: block;
}

.result-details {
  margin-top: 40rpx;
  width: 100%;
  padding-top: 30rpx;
  border-top: 2rpx solid #e0e0e0;
}

.detail-item {
  display: flex;
  flex-direction: row;
  margin-bottom: 20rpx;
  align-items: flex-start;
}

.detail-label {
  font-family: 'NotoSerifSC', serif;
  font-size: 26rpx;
  color: #8b0000;
  font-weight: bold;
  min-width: 140rpx;
  flex-shrink: 0;
}

.detail-value {
  font-family: 'NotoSerifSC', serif;
  font-size: 26rpx;
  color: #333333;
  flex: 1;
}

.interpretation-section,
.advice-section {
  margin-top: 30rpx;
  padding-top: 20rpx;
  border-top: 1rpx solid #e0e0e0;
}

.interpretation-label,
.advice-label {
  font-family: 'ZhiMangXing', cursive;
  font-size: 32rpx;
  color: #8b0000;
  margin-bottom: 15rpx;
  display: block;
}

.interpretation-text,
.advice-text {
  font-family: 'NotoSerifSC', serif;
  font-size: 26rpx;
  color: #333333;
  line-height: 1.8;
  display: block;
}
</style>
