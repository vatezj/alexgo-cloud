<!--
  生辰信息表单（迁自 luckyU bazhi/components/BirthForm.vue）
  - picker/提交钮为原生控件（源稿未用旧 UI 库 → 无需换库），样式原样
  - 新增 disabled prop：30503 限流时页面禁用提交至次日本地日期翻转（Task 5 绑定）
  - 数据逻辑走迁入的 composables/useBirthForm（getBirthData → Task 1 BaziReq）
-->
<script setup lang="ts">
import { useBirthForm } from '../../composables/useBirthForm'

const props = defineProps<{
  onSubmit: () => void
  /** 30503 限流锁定：禁用提交（页面侧文案表负责 toast）。 */
  disabled?: boolean
}>()

const {
  years,
  months,
  days,
  shichenList,
  yearIndex,
  monthIndex,
  dayIndex,
  shichenIndex,
  onYearChange,
  onMonthChange,
  onDayChange,
  onShichenChange,
  getBirthInfo,
  getBirthData,
} = useBirthForm()

function handleClick() {
  if (props.disabled)
    return
  props.onSubmit()
}

defineExpose({
  getBirthInfo,
  getBirthData,
})
</script>

<template>
  <view class="form-group">
    <text class="form-label">
      请输入您的生辰信息
    </text>
    <view class="date-inputs">
      <picker mode="selector" :range="years" :value="yearIndex" @change="onYearChange">
        <view class="picker-cell">
          <text class="picker-val">
            {{ years[yearIndex] }}
          </text>
          <text class="picker-suffix">
            年
          </text>
        </view>
      </picker>
      <picker mode="selector" :range="months" :value="monthIndex" @change="onMonthChange">
        <view class="picker-cell">
          <text class="picker-val">
            {{ months[monthIndex] }}
          </text>
          <text class="picker-suffix">
            月
          </text>
        </view>
      </picker>
      <picker mode="selector" :range="days" :value="dayIndex" @change="onDayChange">
        <view class="picker-cell">
          <text class="picker-val">
            {{ days[dayIndex] }}
          </text>
          <text class="picker-suffix">
            日
          </text>
        </view>
      </picker>
    </view>
  </view>

  <view class="form-group time-group">
    <picker mode="selector" :range="shichenList" :value="shichenIndex" @change="onShichenChange">
      <view class="picker-cell picker-cell-time">
        <text class="picker-val">
          {{ shichenList[shichenIndex] }}
        </text>
      </view>
    </picker>
  </view>

  <view
    class="submit-btn"
    :class="{ 'submit-btn-disabled': disabled }"
    hover-class="submit-btn-hover"
    @click="handleClick"
  >
    <view class="submit-btn-inner-border" />
    <text class="submit-btn-text">
      排 盘 测 算
    </text>
  </view>
</template>

<style scoped lang="scss">
/* ========== 表单 ========== */
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
.date-inputs {
  display: flex;
  flex-direction: row;
  gap: 20rpx;
}
.date-inputs > picker {
  flex: 1;
}
.date-inputs > picker:first-child {
  flex: 1.2;
}
.picker-cell {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: center;
  border-bottom: 2rpx solid #5c4033;
  padding: 20rpx 10rpx;
  position: relative;
}
.picker-val {
  font-family: 'NotoSerifSC', serif;
  font-size: 32rpx;
  color: #1a1a1a;
  text-align: center;
}
.picker-suffix {
  font-size: 24rpx;
  color: #888888;
  margin-left: 6rpx;
}
.time-group {
  display: flex;
  justify-content: center;
}
.time-group > picker {
  width: 50%;
}
.picker-cell-time {
  justify-content: center;
}

/* ========== 提交按钮 ========== */
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
