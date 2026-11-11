<script setup lang="ts">
/**
 * 趣味方位模块 · 自绘 nav（六页共用；骨架同 manual/结算页）
 *
 * - 左返回 → emit('back')，回退语义由页面决定（模块首页→我的，子页→栈内）
 * - H5 装饰假胶囊 #ifndef MP-WEIXIN；MP 右侧让位真胶囊（--nav-right-pad）
 * - 色值全 token；类名前缀 fcn-（避 UnoCSS 撞名清单）
 */
defineProps<{
  /** 主标题（Noto Serif） */
  title: string
  /** 副题 caption，默认「趣味方位参考 · 仅供娱乐」 */
  sub?: string
}>()

const emit = defineEmits<{
  (e: 'back'): void
}>()
</script>

<template>
  <view class="fcn-nav">
    <view
      class="fcn-side"
      @click="emit('back')"
    >
      <wd-icon
        css-icon
        class="fcn-navicon"
        name="i-carbon-chevron-left"
      />
    </view>
    <view class="fcn-title">
      {{ title }}
      <text class="fcn-sub">
        {{ sub ?? '趣味方位参考 · 仅供娱乐' }}
      </text>
    </view>
    <view class="fcn-side right">
      <!-- #ifndef MP-WEIXIN -->
      <view class="fcn-capsule">
        <text class="fcn-cap-dots">
          •••
        </text>
        <view class="fcn-sep" />
        <view class="fcn-cap-c2">
          <view class="fcn-ring" />
        </view>
      </view>
      <!-- #endif -->
    </view>
  </view>
</template>

<style lang="scss" scoped>
.fcn-nav {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 6px 0 8px;
  /* 小程序：右侧让位微信真胶囊（页面根已水平 16px，变量扣掉避免双倍缩进） */
  padding-right: calc(var(--nav-right-pad, 16px) - 16px);
}

.fcn-side {
  display: flex;
  min-width: 60px;
  align-items: center;
}

.fcn-side.right {
  justify-content: flex-end;
}

.fcn-navicon {
  margin-right: 8px;
  color: var(--brass);
  font-family: 'Noto Serif SC', serif;
  font-size: 26px;
  line-height: 1;
}

.fcn-title {
  color: var(--on-ink);
  font-family: 'Noto Serif SC', serif;
  font-size: 17px;
  font-weight: 500;
  letter-spacing: 0.06em;
  text-align: center;
}

.fcn-sub {
  display: block;
  margin-top: 2px;
  color: var(--dim);
  font-family: 'Noto Sans SC', sans-serif;
  font-size: 10.5px;
  font-weight: 400;
  letter-spacing: 0.02em;
}

/* H5 装饰假胶囊（对齐 manual 页） */
.fcn-capsule {
  display: flex;
  width: 88px;
  height: 32px;
  align-items: center;
  border: 1px solid var(--on-ink-a28);
  border-radius: 999px;
  background: var(--on-ink-a04);
}

.fcn-cap-dots {
  flex: 1;
  color: var(--on-ink);
  font-size: 12px;
  text-align: center;
}

.fcn-sep {
  width: 1px;
  height: 16px;
  background: var(--on-ink-a28);
}

.fcn-cap-c2 {
  display: flex;
  width: 34px;
  height: 30px;
  align-items: center;
  justify-content: center;
}

.fcn-ring {
  width: 14px;
  height: 14px;
  border: 2px solid var(--on-ink);
  border-radius: 50%;
}
</style>
