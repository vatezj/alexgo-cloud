<!--
  求签表单（迁自 luckyU bazhi/components/DivinationForm.vue）
  - 问题 textarea / 分类 chips / 提交钮均原样保留（源稿原生控件，未用旧 UI 库 → 无需换库）
  - 新增 disabled prop：30503 限流时页面禁用提交至次日本地日期翻转（Task 5 绑定）
-->
<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  /** 30503 限流锁定：禁用提交（页面侧文案表负责 toast）。 */
  disabled?: boolean
}>()

const emit = defineEmits<{
  submit: [data: { question: string, category: string }]
}>()
const question = ref('')
const category = ref('爱情')

const categories = ['爱情', '事业', '财运', '健康', '学业']

// 生成默认问题
function getDefaultQuestion(cat: string) {
  return `今年的${cat}运势如何？`
}

// 判断是否是默认问题格式
function isDefaultQuestion(q: string, cat: string) {
  const defaultQ = getDefaultQuestion(cat)
  return q === defaultQ || q === defaultQ.replace('？', '?') || q === defaultQ.replace('？', '')
}

// 监听分类变化，自动更新问题
watch(category, (newCategory, oldCategory) => {
  // 如果问题为空，或者是之前分类的默认问题，则自动更新为新分类的问题
  if (!question.value.trim() || (oldCategory && isDefaultQuestion(question.value.trim(), oldCategory))) {
    question.value = getDefaultQuestion(newCategory)
  }
}, { immediate: true })

function handleSubmit() {
  if (props.disabled)
    return

  // 如果问题为空，使用默认问题
  const finalQuestion = question.value.trim() || getDefaultQuestion(category.value)

  emit('submit', {
    question: finalQuestion,
    category: category.value,
  })
}
</script>

<template>
  <view class="divination-form">
    <view class="form-group">
      <text class="form-label">
        请输入您的问题
      </text>
      <textarea
        v-model="question"
        class="question-input"
        :placeholder="`例如：${getDefaultQuestion(category)}`"
        :maxlength="200"
        auto-height
      />
    </view>

    <view class="form-group">
      <text class="form-label">
        选择分类
      </text>
      <view class="category-list">
        <view
          v-for="cat in categories"
          :key="cat"
          class="category-item"
          :class="{ active: category === cat }"
          @click="category = cat"
        >
          <text class="category-text">
            {{ cat }}
          </text>
        </view>
      </view>
    </view>

    <view
      class="submit-btn"
      :class="{ 'submit-btn-disabled': disabled }"
      hover-class="submit-btn-hover"
      @click="handleSubmit"
    >
      <view class="submit-btn-inner-border" />
      <text class="submit-btn-text">
        开始求签
      </text>
    </view>
  </view>
</template>

<style scoped lang="scss">
.divination-form {
  width: 100%;
}

.form-group {
  margin-bottom: 30rpx;
}

.form-label {
  display: block;
  font-family: 'NotoSerifSC', serif;
  color: #8b0000;
  font-size: 28rpx;
  margin-bottom: 16rpx;
  font-weight: bold;
}

.question-input {
  width: 100%;
  min-height: 120rpx;
  padding: 20rpx;
  border: 2rpx solid #5c4033;
  border-radius: 8rpx;
  font-family: 'NotoSerifSC', serif;
  font-size: 28rpx;
  color: #1a1a1a;
  background-color: #ffffff;
  box-sizing: border-box;
}

.category-list {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
}

.category-item {
  padding: 16rpx 32rpx;
  border: 2rpx solid #5c4033;
  border-radius: 40rpx;
  background-color: #ffffff;
  transition: all 0.3s ease;

  &.active {
    background-color: #8b0000;
    border-color: #8b0000;
  }
}

.category-text {
  font-family: 'NotoSerifSC', serif;
  font-size: 26rpx;
  color: #5c4033;

  .category-item.active & {
    color: #d4af37;
  }
}

.submit-btn {
  width: 100%;
  background-color: #8b0000;
  padding: 24rpx 0;
  margin-top: 20rpx;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: center;
}

.submit-btn-inner-border {
  position: absolute;
  top: 4rpx;
  bottom: 4rpx;
  left: 4rpx;
  right: 4rpx;
  border: 2rpx solid #d4af37;
  pointer-events: none;
}

.submit-btn-text {
  font-family: 'ZhiMangXing', cursive;
  font-size: 40rpx;
  color: #d4af37;
  letter-spacing: 4rpx;
}

.submit-btn-hover {
  opacity: 0.85;
}

.submit-btn-disabled {
  opacity: 0.5;
}
</style>
